// Package postgres is the production domain.Store backed by PostgreSQL (pgx). It applies on-chain
// truth atomically: each ApplyChainEvent/Reconcile runs in a transaction that locks the payment
// row (SELECT ... FOR UPDATE) so concurrent events/reconciles cannot double-advance (A.7).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
)

// Store is a pgx-backed domain.Store.
type Store struct {
	pool *pgxpool.Pool
}

// New connects a pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

const selectColumns = `id, paylink_id, rail, status, last_event_seq, created_at, updated_at`

// row is the minimal scan surface shared by pool and tx QueryRow results.
type row interface {
	Scan(dest ...any) error
}

func scanPayment(r row) (domain.Payment, error) {
	var (
		p      domain.Payment
		status string
		seq    int64
	)
	if err := r.Scan(&p.ID, &p.PayLinkID, &p.Rail, &status, &seq, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.Payment{}, err
	}
	p.Status = lifecycle.State(status)
	p.LastEventSeq = uint64(seq)
	return p, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreatePayment inserts a payment, mapping the unique-paylink violation to ErrPaymentExists.
func (s *Store) CreatePayment(ctx context.Context, p domain.Payment) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO payment.payments (id, paylink_id, rail, status, last_event_seq, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		p.ID, p.PayLinkID, p.Rail, string(p.Status), int64(p.LastEventSeq), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrPaymentExists
		}
		return err
	}
	return nil
}

// GetPayment returns the payment by id, or domain.ErrNotFound.
func (s *Store) GetPayment(ctx context.Context, id string) (domain.Payment, error) {
	p, err := scanPayment(s.pool.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM payment.payments WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrNotFound
	}
	return p, err
}

// GetPaymentByPayLink returns the payment for a paylink id, or domain.ErrNotFound.
func (s *Store) GetPaymentByPayLink(ctx context.Context, paylinkID string) (domain.Payment, error) {
	p, err := scanPayment(s.pool.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM payment.payments WHERE paylink_id=$1`, paylinkID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrNotFound
	}
	return p, err
}

// ApplyChainEvent advances the payment atomically and idempotently. The (paylink_id, seq) insert
// dedupes redelivered events; the FOR UPDATE lock serializes concurrent applies.
func (s *Store) ApplyChainEvent(ctx context.Context, ev domain.ChainEventRef, project domain.ProjectFn) (domain.Payment, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	p, err := scanPayment(tx.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM payment.payments WHERE paylink_id=$1 FOR UPDATE`, ev.PayLinkID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, domain.ErrNotFound
	}
	if err != nil {
		return domain.Payment{}, false, err
	}

	ct, err := tx.Exec(ctx,
		`INSERT INTO payment.applied_chain_events (paylink_id, seq, kind, tx_hash)
		 VALUES ($1,$2,$3,$4) ON CONFLICT (paylink_id, seq) DO NOTHING`,
		ev.PayLinkID, int64(ev.Seq), ev.Kind, ev.TxHash)
	if err != nil {
		return domain.Payment{}, false, err
	}
	if ct.RowsAffected() == 0 {
		// Duplicate event — already applied; commit to release the lock, no state change.
		return p, false, tx.Commit(ctx)
	}

	next, changed, perr := project(p.Status)
	if perr != nil {
		// Record the event (dedupe row is inserted) but make no state change.
		if cerr := tx.Commit(ctx); cerr != nil {
			return p, false, cerr
		}
		return p, false, perr
	}

	seq := p.LastEventSeq
	if ev.Seq > seq {
		seq = ev.Seq
	}
	if !changed {
		if seq != p.LastEventSeq {
			if _, err := tx.Exec(ctx, `UPDATE payment.payments SET last_event_seq=$1 WHERE id=$2`, int64(seq), p.ID); err != nil {
				return domain.Payment{}, false, err
			}
			p.LastEventSeq = seq
		}
		return p, false, tx.Commit(ctx)
	}

	if err := tx.QueryRow(ctx,
		`UPDATE payment.payments SET status=$1, last_event_seq=$2, updated_at=now() WHERE id=$3 RETURNING updated_at`,
		string(next), int64(seq), p.ID).Scan(&p.UpdatedAt); err != nil {
		return domain.Payment{}, false, err
	}
	p.Status = next
	p.LastEventSeq = seq
	return p, true, tx.Commit(ctx)
}

// Reconcile advances the payment toward on-chain truth on the read path (no event dedupe row).
func (s *Store) Reconcile(ctx context.Context, paylinkID string, project domain.ProjectFn) (domain.Payment, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	p, err := scanPayment(tx.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM payment.payments WHERE paylink_id=$1 FOR UPDATE`, paylinkID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, domain.ErrNotFound
	}
	if err != nil {
		return domain.Payment{}, false, err
	}

	next, changed, perr := project(p.Status)
	if perr != nil {
		return p, false, perr
	}
	if !changed {
		return p, false, tx.Commit(ctx)
	}

	if err := tx.QueryRow(ctx,
		`UPDATE payment.payments SET status=$1, updated_at=now() WHERE id=$2 RETURNING updated_at`,
		string(next), p.ID).Scan(&p.UpdatedAt); err != nil {
		return domain.Payment{}, false, err
	}
	p.Status = next
	return p, true, tx.Commit(ctx)
}
