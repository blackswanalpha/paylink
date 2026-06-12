// Package postgres is the production domain.Store backed by PostgreSQL (pgx). It applies
// condition/funding decisions atomically: each Mutate/ApplyFunding runs in a transaction that
// locks the escrow row (SELECT ... FOR UPDATE) so concurrent confirms/funding events cannot
// double-advance (A.7). ApplyFunding additionally writes a DbDedupe (work17) row to
// escrow.processed_events on the SAME transaction, so a redelivered chain.paylink.verified
// event applies its effect exactly once. The sweeper paths are single-statement CAS updates
// (UPDATE ... WHERE state='WAITING' ... RETURNING).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/fsm"
)

// dedupeScope is the DbDedupe scope for funding events.
const dedupeScope = "chain.paylink.verified"

// Store is a pgx-backed domain.Store.
type Store struct {
	pool   *pgxpool.Pool
	dedupe *idempotency.DbDedupe
}

// New connects a pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, dedupe: idempotency.NewDbDedupe("escrow.processed_events")}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

const selectColumns = `id, pl_id, creator_addr, payee_addr, refund_to, amount::text, currency,
	condition_type, condition_params, state, funded, funded_tx_hash, release_at, timeout_at,
	dispute_reason, created_at, updated_at`

// row is the minimal scan surface shared by pool and tx QueryRow results.
type row interface {
	Scan(dest ...any) error
}

func scanEscrow(r row) (domain.Escrow, error) {
	var (
		e      domain.Escrow
		state  string
		params []byte
	)
	if err := r.Scan(&e.ID, &e.PLID, &e.CreatorAddr, &e.PayeeAddr, &e.RefundTo, &e.Amount,
		&e.Currency, &e.ConditionType, &params, &state, &e.Funded, &e.FundedTxHash,
		&e.ReleaseAt, &e.TimeoutAt, &e.DisputeReason, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return domain.Escrow{}, err
	}
	e.State = fsm.State(state)
	if len(params) > 0 {
		if err := json.Unmarshal(params, &e.ConditionParams); err != nil {
			return domain.Escrow{}, err
		}
	}
	return e, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateEscrow inserts an escrow, mapping the unique-pl_id violation to ErrEscrowExists.
func (s *Store) CreateEscrow(ctx context.Context, e domain.Escrow) error {
	params, err := json.Marshal(e.ConditionParams)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO escrow.escrows (id, pl_id, creator_addr, payee_addr, refund_to, amount,
		   currency, condition_type, condition_params, state, funded, funded_tx_hash, release_at,
		   timeout_at, dispute_reason, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		e.ID, e.PLID, e.CreatorAddr, e.PayeeAddr, e.RefundTo, e.Amount, e.Currency,
		e.ConditionType, params, string(e.State), e.Funded, e.FundedTxHash, e.ReleaseAt,
		e.TimeoutAt, e.DisputeReason, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEscrowExists
		}
		return err
	}
	return nil
}

// GetEscrow returns the escrow by id, or domain.ErrNotFound.
func (s *Store) GetEscrow(ctx context.Context, id string) (domain.Escrow, error) {
	e, err := scanEscrow(s.pool.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM escrow.escrows WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Escrow{}, domain.ErrNotFound
	}
	return e, err
}

// ListEscrows returns the creator's escrows, optionally filtered by state, most recent first.
func (s *Store) ListEscrows(ctx context.Context, creatorAddr, state string, limit int) ([]domain.Escrow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+selectColumns+` FROM escrow.escrows
		 WHERE creator_addr=$1 AND ($2 = '' OR state=$2)
		 ORDER BY created_at DESC, id DESC LIMIT $3`, creatorAddr, state, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows, limit)
}

// Mutate locks the escrow by id, calls fn with the row + recorded approvals, and applies the
// returned Update atomically.
func (s *Store) Mutate(ctx context.Context, id string, fn domain.MutateFn) (domain.Escrow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Escrow{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	e, approvals, err := lockEscrow(ctx, tx, `id=$1`, id)
	if err != nil {
		return domain.Escrow{}, err
	}
	up, err := fn(e, approvals)
	if err != nil {
		return domain.Escrow{}, err
	}
	if err := applyUpdate(ctx, tx, &e, up); err != nil {
		return domain.Escrow{}, err
	}
	return e, tx.Commit(ctx)
}

// ApplyFunding locks the escrow by pl_id and runs fn under a DbDedupe guard keyed on
// (scope="chain.paylink.verified", key=plID+":"+txHash). The dedupe row and fn's update commit
// together; a duplicate returns applied=false with no changes; an fn error rolls both back.
func (s *Store) ApplyFunding(ctx context.Context, plID, txHash string, fn domain.MutateFn) (domain.Escrow, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Escrow{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	e, approvals, err := lockEscrow(ctx, tx, `pl_id=$1`, plID)
	if err != nil {
		return domain.Escrow{}, false, err
	}

	applied, err := s.dedupe.RunOnce(ctx, tx, dedupeScope, plID+":"+txHash, func() error {
		up, ferr := fn(e, approvals)
		if ferr != nil {
			return ferr
		}
		return applyUpdate(ctx, tx, &e, up)
	})
	if err != nil {
		return domain.Escrow{}, false, err
	}
	return e, applied, tx.Commit(ctx)
}

// lockEscrow selects one escrow FOR UPDATE by the given predicate and loads its approvals.
func lockEscrow(ctx context.Context, tx pgx.Tx, where, arg string) (domain.Escrow, []string, error) {
	e, err := scanEscrow(tx.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM escrow.escrows WHERE `+where+` FOR UPDATE`, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Escrow{}, nil, domain.ErrNotFound
	}
	if err != nil {
		return domain.Escrow{}, nil, err
	}
	rows, err := tx.Query(ctx,
		`SELECT approver_addr FROM escrow.approvals WHERE escrow_id=$1 ORDER BY approved_at, approver_addr`, e.ID)
	if err != nil {
		return domain.Escrow{}, nil, err
	}
	defer rows.Close()
	var approvals []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return domain.Escrow{}, nil, err
		}
		approvals = append(approvals, a)
	}
	return e, approvals, rows.Err()
}

// applyUpdate applies an Update on the caller's transaction and refreshes e in place.
func applyUpdate(ctx context.Context, tx pgx.Tx, e *domain.Escrow, up domain.Update) error {
	if up.AddApproval != "" {
		if _, err := tx.Exec(ctx,
			`INSERT INTO escrow.approvals (escrow_id, approver_addr) VALUES ($1,$2)
			 ON CONFLICT (escrow_id, approver_addr) DO NOTHING`, e.ID, up.AddApproval); err != nil {
			return err
		}
	}
	if !up.SetFunded && up.SetState == "" && up.DisputeReason == "" {
		return nil
	}
	if err := tx.QueryRow(ctx,
		`UPDATE escrow.escrows SET
		   funded         = funded OR $2,
		   funded_tx_hash = CASE WHEN $2 AND NOT funded THEN $3 ELSE funded_tx_hash END,
		   state          = CASE WHEN $4 <> '' THEN $4 ELSE state END,
		   dispute_reason = CASE WHEN $5 <> '' THEN $5 ELSE dispute_reason END,
		   updated_at     = now()
		 WHERE id=$1
		 RETURNING funded, funded_tx_hash, state, dispute_reason, updated_at`,
		e.ID, up.SetFunded, up.FundedTxHash, string(up.SetState), up.DisputeReason).
		Scan(&e.Funded, &e.FundedTxHash, &stateScanner{&e.State}, &e.DisputeReason, &e.UpdatedAt); err != nil {
		return err
	}
	return nil
}

// stateScanner scans a text column into an fsm.State.
type stateScanner struct{ s *fsm.State }

func (ss *stateScanner) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*ss.s = fsm.State(v)
		return nil
	case []byte:
		*ss.s = fsm.State(v)
		return nil
	}
	return errors.New("state: unsupported scan source")
}

// ReleaseDueTimeLocks CAS-releases funded time_lock escrows whose release_at has passed. The
// WHERE state='WAITING' clause is the guard — DISPUTED/advanced rows are untouched.
func (s *Store) ReleaseDueTimeLocks(ctx context.Context, now time.Time) ([]domain.Escrow, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE escrow.escrows SET state=$1, updated_at=now()
		 WHERE state=$2 AND funded AND condition_type=$3 AND release_at IS NOT NULL AND release_at <= $4
		 RETURNING `+selectColumns,
		string(fsm.StateReleased), string(fsm.StateWaiting), domain.ConditionTimeLock, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows, 0)
}

// RefundTimedOut CAS-refunds WAITING escrows whose timeout_at has passed (works for unfunded
// escrows too; the funded flag rides along in the returned rows for the event payload).
func (s *Store) RefundTimedOut(ctx context.Context, now time.Time) ([]domain.Escrow, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE escrow.escrows SET state=$1, updated_at=now()
		 WHERE state=$2 AND timeout_at <= $3
		 RETURNING `+selectColumns,
		string(fsm.StateRefunded), string(fsm.StateWaiting), now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows, 0)
}

func collect(rows pgx.Rows, capHint int) ([]domain.Escrow, error) {
	out := make([]domain.Escrow, 0, capHint)
	for rows.Next() {
		e, err := scanEscrow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
