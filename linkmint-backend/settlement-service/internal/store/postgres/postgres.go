// Package postgres is the production domain.Store backed by PostgreSQL (pgx). Each mutating method
// runs in one transaction that joins the business-state write, the DbDedupe (work17) mark, and the
// balanced ledger.Post (work16, A.6) so they commit or roll back together. Reads are pool queries.
//
// NON-CUSTODIAL (A.1): no balances are held; ledger entries record flows for reconciliation only.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	idempotency "github.com/paylink/idempotency-go"
	ledger "github.com/paylink/ledger-go"

	"github.com/paylink/settlement-service/internal/domain"
)

// DbDedupe scopes (one per consumed event family).
const (
	scopeVerified = "chain.paylink.verified"
	scopeFee      = "chain.fee.collected"
	scopeClawback = "refund.clawback.requested"
	scopeMerchant = "merchant.onboarded"
	scopeBank     = "merchant.bank_account.verified"
)

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
	return &Store{pool: pool, dedupe: idempotency.NewDbDedupe("settlement.processed_events")}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ── ledger account naming ──────────────────────────────────────────────────────────────────────

func acctClearing(ccy string) string        { return "settlement:clearing:" + ccy }
func acctMerchantPayable(key string) string { return "merchant_payable:" + key }
func acctFeePlatform(ccy string) string     { return "fee:platform:" + ccy }
func acctFeeChain(ccy string) string        { return "fee:chain:" + ccy }

func leg(account string, dir ledger.Direction, amount *big.Int, ccy string) ledger.Leg {
	return ledger.Leg{Account: account, Direction: dir, Amount: amount, Currency: ccy}
}

// ── RecordVerified ───────────────────────────────────────────────────────────────────────────────

// RecordVerified upserts the OPEN settlement for the verified PayLink's period, inserts the paylink
// item, and posts the gross ledger entry — all under a DbDedupe guard on the pl_id.
func (s *Store) RecordVerified(ctx context.Context, in domain.VerifiedRecord) (domain.VerifiedOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.VerifiedOutcome{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var (
		opened     bool
		settlement domain.Settlement
	)
	net := new(big.Int).Sub(in.Gross, in.PlatformFee) // net before chain fee = gross - platform fee

	applied, err := s.dedupe.RunOnce(ctx, tx, scopeVerified, in.PLID, func() error {
		sID, isNew, uerr := upsertOpenSettlement(ctx, tx, in.MerchantKey, in.Currency, in.SettlementDate, in.CutoffAt)
		if uerr != nil {
			return uerr
		}
		opened = isNew

		if _, ierr := tx.Exec(ctx,
			`INSERT INTO settlement.settlement_items
			   (id, settlement_id, pl_id, kind, gross, platform_fee, chain_fee, net, verified_tx_hash)
			 VALUES ($1,$2,$3,'paylink',$4::numeric,$5::numeric,0,$6::numeric,$7)
			 ON CONFLICT (pl_id) WHERE kind = 'paylink' DO NOTHING`,
			uuid.NewString(), sID, in.PLID, in.Gross.String(), in.PlatformFee.String(), net.String(), in.TxHash); ierr != nil {
			return ierr
		}

		// Ledger (A.6): DR clearing (gross) / CR merchant_payable (net) [/ CR fee:platform (platform fee)].
		legs := []ledger.Leg{
			leg(acctClearing(in.Currency), ledger.DR, in.Gross, in.Currency),
			leg(acctMerchantPayable(in.MerchantKey), ledger.CR, net, in.Currency),
		}
		if in.PlatformFee.Sign() > 0 {
			legs = append(legs, leg(acctFeePlatform(in.Currency), ledger.CR, in.PlatformFee, in.Currency))
		}
		if _, perr := ledger.Post(ctx, tx, ledger.PostingInput{
			Entries: legs, PLID: in.PLID, Note: "settlement gross " + in.PLID,
		}); perr != nil {
			return perr
		}

		s2, berr := bumpSettlement(ctx, tx, sID, in.Gross, in.PlatformFee, big.NewInt(0), net)
		if berr != nil {
			return berr
		}
		settlement = s2
		return nil
	})
	if err != nil {
		return domain.VerifiedOutcome{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.VerifiedOutcome{}, err
	}
	return domain.VerifiedOutcome{Applied: applied, Opened: opened && applied, Settlement: settlement}, nil
}

// ── RecordFee ────────────────────────────────────────────────────────────────────────────────────

// RecordFee attaches the chain fee to the settled PayLink's item and posts the fee ledger entry. If
// no paylink item exists yet for the pl_id (verified not processed — anomalous), it returns
// Found=false WITHOUT touching the dedupe table, so a later redelivery can still apply it.
func (s *Store) RecordFee(ctx context.Context, in domain.FeeRecord) (domain.FeeOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.FeeOutcome{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var (
		itemID, settlementID, merchantKey, currency string
	)
	err = tx.QueryRow(ctx,
		`SELECT i.id, i.settlement_id, s.merchant_key, s.currency
		   FROM settlement.settlement_items i
		   JOIN settlement.settlements s ON s.id = i.settlement_id
		  WHERE i.pl_id = $1 AND i.kind = 'paylink'
		  FOR UPDATE OF i`, in.PLID).Scan(&itemID, &settlementID, &merchantKey, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FeeOutcome{Found: false}, nil
	}
	if err != nil {
		return domain.FeeOutcome{}, err
	}

	applied, err := s.dedupe.RunOnce(ctx, tx, scopeFee, in.PLID, func() error {
		if _, uerr := tx.Exec(ctx,
			`UPDATE settlement.settlement_items
			    SET chain_fee = chain_fee + $2::numeric, net = net - $2::numeric
			  WHERE id = $1`, itemID, in.ChainFee.String()); uerr != nil {
			return uerr
		}
		if in.ChainFee.Sign() > 0 {
			if _, perr := ledger.Post(ctx, tx, ledger.PostingInput{
				Entries: []ledger.Leg{
					leg(acctMerchantPayable(merchantKey), ledger.DR, in.ChainFee, currency),
					leg(acctFeeChain(currency), ledger.CR, in.ChainFee, currency),
				},
				PLID: in.PLID, Note: "settlement chain fee " + in.PLID,
			}); perr != nil {
				return perr
			}
		}
		negFee := new(big.Int).Neg(in.ChainFee)
		_, berr := bumpSettlement(ctx, tx, settlementID, big.NewInt(0), big.NewInt(0), in.ChainFee, negFee)
		return berr
	})
	if err != nil {
		return domain.FeeOutcome{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FeeOutcome{}, err
	}
	return domain.FeeOutcome{Applied: applied, Found: true}, nil
}

// ── RecordClawback ───────────────────────────────────────────────────────────────────────────────

// RecordClawback records a refund clawback as a negative offset against the merchant's OPEN
// settlement for the clawback period, resolving the merchant from the clawed-back pl_id's item.
func (s *Store) RecordClawback(ctx context.Context, in domain.ClawbackRecord) (domain.ClawbackOutcome, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ClawbackOutcome{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var merchantKey, currency string
	err = tx.QueryRow(ctx,
		`SELECT s.merchant_key, s.currency
		   FROM settlement.settlement_items i
		   JOIN settlement.settlements s ON s.id = i.settlement_id
		  WHERE i.pl_id = $1 AND i.kind = 'paylink'`, in.PLID).Scan(&merchantKey, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClawbackOutcome{Found: false}, nil
	}
	if err != nil {
		return domain.ClawbackOutcome{}, err
	}

	applied, err := s.dedupe.RunOnce(ctx, tx, scopeClawback, in.RefundID, func() error {
		sID, _, uerr := upsertOpenSettlement(ctx, tx, merchantKey, currency, in.SettlementDate, in.CutoffAt)
		if uerr != nil {
			return uerr
		}
		negNet := new(big.Int).Neg(in.Amount)
		if _, ierr := tx.Exec(ctx,
			`INSERT INTO settlement.settlement_items (id, settlement_id, pl_id, kind, gross, net)
			 VALUES ($1,$2,$3,'clawback',0,$4::numeric)`,
			uuid.NewString(), sID, in.PLID, negNet.String()); ierr != nil {
			return ierr
		}
		if _, cerr := tx.Exec(ctx,
			`INSERT INTO settlement.clawbacks (id, refund_id, merchant_key, pl_id, amount, currency, settlement_id)
			 VALUES ($1,$2,$3,$4,$5::numeric,$6,$7)`,
			uuid.NewString(), in.RefundID, merchantKey, in.PLID, in.Amount.String(), currency, sID); cerr != nil {
			return cerr
		}
		// Ledger (A.6): recover from merchant payable back to clearing.
		if _, perr := ledger.Post(ctx, tx, ledger.PostingInput{
			Entries: []ledger.Leg{
				leg(acctMerchantPayable(merchantKey), ledger.DR, in.Amount, currency),
				leg(acctClearing(currency), ledger.CR, in.Amount, currency),
			},
			PLID: in.PLID, Note: "settlement clawback " + in.RefundID,
		}); perr != nil {
			return perr
		}
		_, berr := bumpSettlement(ctx, tx, sID, big.NewInt(0), big.NewInt(0), big.NewInt(0), negNet)
		return berr
	})
	if err != nil {
		return domain.ClawbackOutcome{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ClawbackOutcome{}, err
	}
	return domain.ClawbackOutcome{Applied: applied, Found: true}, nil
}

// ── projections ──────────────────────────────────────────────────────────────────────────────────

// UpsertMerchant upserts the merchant projection (deduped on merchant_id).
func (s *Store) UpsertMerchant(ctx context.Context, m domain.Merchant) (bool, error) {
	return s.dedupeUpsert(ctx, scopeMerchant, m.MerchantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO settlement.merchant_directory (merchant_id, tz, default_rail, status)
			 VALUES ($1,$2,$3,$4)
			 ON CONFLICT (merchant_id) DO UPDATE
			   SET tz = EXCLUDED.tz, default_rail = EXCLUDED.default_rail,
			       status = EXCLUDED.status, updated_at = now()`,
			m.MerchantID, m.TZ, m.DefaultRail, m.Status)
		return err
	})
}

// UpsertBankAccount upserts the bank-account projection (deduped on bank_account_id).
func (s *Store) UpsertBankAccount(ctx context.Context, b domain.BankAccount) (bool, error) {
	return s.dedupeUpsert(ctx, scopeBank, b.BankAccountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO settlement.bank_accounts (bank_account_id, merchant_id, rail, currency, status)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (bank_account_id) DO UPDATE
			   SET merchant_id = EXCLUDED.merchant_id, rail = EXCLUDED.rail,
			       currency = EXCLUDED.currency, status = EXCLUDED.status, updated_at = now()`,
			b.BankAccountID, b.MerchantID, b.Rail, b.Currency, b.Status)
		return err
	})
}

// dedupeUpsert runs fn under a DbDedupe guard on (scope, key) in its own transaction.
func (s *Store) dedupeUpsert(ctx context.Context, scope, key string, fn func(pgx.Tx) error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	applied, err := s.dedupe.RunOnce(ctx, tx, scope, key, func() error { return fn(tx) })
	if err != nil {
		return false, err
	}
	return applied, tx.Commit(ctx)
}

// ── scheduling ───────────────────────────────────────────────────────────────────────────────────

// CloseDueSettlements CAS-transitions OPEN settlements past their cutoff to CLOSED.
func (s *Store) CloseDueSettlements(ctx context.Context, now time.Time) ([]domain.Settlement, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE settlement.settlements SET status='CLOSED', closed_at=now()
		  WHERE status='OPEN' AND cutoff_at <= $1
		  RETURNING `+settlementColumns, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSettlements(rows)
}

// SchedulePayouts creates+instructs a payout for each CLOSED settlement that has no payout yet and
// whose net meets the per-currency minimum. Below the minimum (or net<=0) the settlement is skipped
// (it stays CLOSED — carried, not paid). Each payout is created in its own transaction.
func (s *Store) SchedulePayouts(ctx context.Context, now time.Time, opts domain.ScheduleOpts) ([]domain.Payout, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+settlementColumnsPrefixed("s")+`
		   FROM settlement.settlements s
		   LEFT JOIN settlement.payouts p ON p.settlement_id = s.id
		  WHERE s.status='CLOSED' AND p.id IS NULL
		  ORDER BY s.cutoff_at
		  LIMIT 500`)
	if err != nil {
		return nil, err
	}
	candidates, err := scanSettlements(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}

	rail := opts.DefaultRail
	if rail == "" {
		rail = "unknown"
	}
	var out []domain.Payout
	for _, st := range candidates {
		min := big.NewInt(0)
		if opts.MinPayoutFor != nil {
			if m := opts.MinPayoutFor(st.Currency); m != nil {
				min = m
			}
		}
		if st.Net.Sign() <= 0 || st.Net.Cmp(min) < 0 {
			continue // below minimum / nothing to pay — carry forward
		}
		p, perr := s.instructPayout(ctx, st, rail, now)
		if perr != nil {
			return out, perr
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, nil
}

// instructPayout creates an INSTRUCTED payout for a settlement (idempotent on settlement_id). Returns
// nil if a payout already exists for the settlement (lost the race / re-scheduled).
func (s *Store) instructPayout(ctx context.Context, st domain.Settlement, rail string, now time.Time) (*domain.Payout, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	p, err := insertInstructedPayout(ctx, tx, st, rail, now)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return p, tx.Commit(ctx)
}

// CreatePayout creates+instructs an on-demand payout for a CLOSED, merchant-owned settlement.
func (s *Store) CreatePayout(ctx context.Context, settlementID, merchantKey, defaultRail string) (domain.Payout, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Payout{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	st, err := scanSettlement(tx.QueryRow(ctx,
		`SELECT `+settlementColumns+` FROM settlement.settlements WHERE id=$1 FOR UPDATE`, settlementID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payout{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Payout{}, err
	}
	if st.MerchantKey != merchantKey {
		return domain.Payout{}, domain.ErrNotFound // do not leak another merchant's settlement
	}
	if st.Status != domain.StatusClosed {
		return domain.Payout{}, fmt.Errorf("%w: settlement must be CLOSED to pay out (is %s)", domain.ErrInvalidState, st.Status)
	}
	if st.Net.Sign() <= 0 {
		return domain.Payout{}, fmt.Errorf("%w: settlement net must be positive", domain.ErrInvalidAmount)
	}
	rail := defaultRail
	if rail == "" {
		rail = "unknown"
	}
	p, err := insertInstructedPayout(ctx, tx, st, rail, time.Now())
	if err != nil {
		return domain.Payout{}, err
	}
	if p == nil {
		return domain.Payout{}, fmt.Errorf("%w: a payout already exists for this settlement", domain.ErrInvalidState)
	}
	return *p, tx.Commit(ctx)
}

// insertInstructedPayout inserts a payout in INSTRUCTED state for a settlement (one per settlement).
// Returns nil if a payout already exists (ON CONFLICT DO NOTHING). No ledger entry — a payout is an
// INSTRUCTION (A.1); the ledger outflow is posted when the rail file confirms it PAID.
func insertInstructedPayout(ctx context.Context, tx pgx.Tx, st domain.Settlement, rail string, now time.Time) (*domain.Payout, error) {
	id := uuid.NewString()
	ref := "PO-" + st.ID
	tag, err := tx.Exec(ctx,
		`INSERT INTO settlement.payouts
		   (id, settlement_id, merchant_key, rail, currency, amount, status, reference, scheduled_for, instructed_at)
		 VALUES ($1,$2,$3,$4,$5,$6::numeric,'INSTRUCTED',$7,$8,now())
		 ON CONFLICT (settlement_id) DO NOTHING`,
		id, st.ID, st.MerchantKey, rail, st.Currency, st.Net.String(), ref, st.CutoffAt)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	instructed := now.UTC()
	return &domain.Payout{
		ID: id, SettlementID: st.ID, MerchantKey: st.MerchantKey, Rail: rail,
		Currency: st.Currency, Amount: new(big.Int).Set(st.Net), Status: domain.PayoutInstructed,
		Reference: ref, ScheduledFor: st.CutoffAt, InstructedAt: &instructed,
	}, nil
}

// ── rail-file ingest ─────────────────────────────────────────────────────────────────────────────

// IngestRailFile records the file + lines, matches each line to an INSTRUCTED/SCHEDULED payout by
// (reference, amount, currency), flips matched payouts → PAID and their settlements → PAID (posting
// the payout ledger entry), and leaves unmatched lines for work27.
func (s *Store) IngestRailFile(ctx context.Context, in domain.RailFileInput) (domain.IngestResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.IngestResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx,
		`INSERT INTO settlement.rail_files (id, rail, line_count) VALUES ($1,$2,$3)
		 ON CONFLICT (id) DO NOTHING`, in.FileID, in.Rail, len(in.Lines))
	if err != nil {
		return domain.IngestResult{}, err
	}
	if tag.RowsAffected() == 0 {
		// Already ingested — idempotent no-op; return the stored counts.
		var lineCount, matched int
		if qerr := tx.QueryRow(ctx,
			`SELECT line_count, matched_count FROM settlement.rail_files WHERE id=$1`, in.FileID).
			Scan(&lineCount, &matched); qerr != nil {
			return domain.IngestResult{}, qerr
		}
		if cerr := tx.Commit(ctx); cerr != nil {
			return domain.IngestResult{}, cerr
		}
		return domain.IngestResult{FileID: in.FileID, Rail: in.Rail, LineCount: lineCount, Matched: matched, Unmatched: lineCount - matched}, nil
	}

	var paid []domain.Payout
	matched := 0
	for _, line := range in.Lines {
		amt := line.Amount
		if amt == nil {
			amt = big.NewInt(0)
		}
		p, ok, merr := matchAndPayPayout(ctx, tx, line.Reference, amt, line.Currency)
		if merr != nil {
			return domain.IngestResult{}, merr
		}
		status := domain.LineUnmatched
		if ok {
			status = domain.LineMatched
			matched++
			paid = append(paid, p)
		}
		if _, ierr := tx.Exec(ctx,
			`INSERT INTO settlement.rail_file_lines (file_id, reference, amount, currency, status)
			 VALUES ($1,$2,$3::numeric,$4,$5)`,
			in.FileID, line.Reference, amt.String(), line.Currency, status); ierr != nil {
			return domain.IngestResult{}, ierr
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE settlement.rail_files SET matched_count=$2 WHERE id=$1`, in.FileID, matched); err != nil {
		return domain.IngestResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.IngestResult{}, err
	}
	return domain.IngestResult{
		FileID: in.FileID, Rail: in.Rail, LineCount: len(in.Lines),
		Matched: matched, Unmatched: len(in.Lines) - matched, PaidPayouts: paid,
	}, nil
}

// matchAndPayPayout flips an INSTRUCTED/SCHEDULED payout matching (reference, amount, currency) to
// PAID, marks its settlement PAID, and posts the payout ledger entry. Returns ok=false (no error)
// when no matching open payout exists (already paid / unknown reference).
func matchAndPayPayout(ctx context.Context, tx pgx.Tx, ref string, amount *big.Int, currency string) (domain.Payout, bool, error) {
	p, err := scanPayout(tx.QueryRow(ctx,
		`UPDATE settlement.payouts SET status='PAID', paid_at=now()
		  WHERE reference=$1 AND currency=$2 AND amount=$3::numeric AND status IN ('INSTRUCTED','SCHEDULED')
		  RETURNING `+payoutColumns, ref, currency, amount.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payout{}, false, nil
	}
	if err != nil {
		return domain.Payout{}, false, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE settlement.settlements SET status='PAID' WHERE id=$1`, p.SettlementID); err != nil {
		return domain.Payout{}, false, err
	}
	// Ledger (A.6): the confirmed outflow clears the merchant payable.
	if _, err := ledger.Post(ctx, tx, ledger.PostingInput{
		Entries: []ledger.Leg{
			leg(acctMerchantPayable(p.MerchantKey), ledger.DR, p.Amount, p.Currency),
			leg(acctClearing(p.Currency), ledger.CR, p.Amount, p.Currency),
		},
		Note: "settlement payout " + p.Reference,
	}); err != nil {
		return domain.Payout{}, false, err
	}
	return p, true, nil
}

// ── reads ────────────────────────────────────────────────────────────────────────────────────────

// GetSettlement returns a merchant-owned settlement with its items.
func (s *Store) GetSettlement(ctx context.Context, id, merchantKey string) (domain.Settlement, []domain.SettlementItem, error) {
	st, err := scanSettlement(s.pool.QueryRow(ctx,
		`SELECT `+settlementColumns+` FROM settlement.settlements WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settlement{}, nil, domain.ErrNotFound
	}
	if err != nil {
		return domain.Settlement{}, nil, err
	}
	if st.MerchantKey != merchantKey {
		return domain.Settlement{}, nil, domain.ErrNotFound
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, settlement_id, pl_id, kind, gross::text, platform_fee::text, chain_fee::text,
		        net::text, verified_tx_hash, created_at
		   FROM settlement.settlement_items WHERE settlement_id=$1 ORDER BY created_at, id`, id)
	if err != nil {
		return domain.Settlement{}, nil, err
	}
	defer rows.Close()
	var items []domain.SettlementItem
	for rows.Next() {
		var (
			it                 domain.SettlementItem
			gross, pf, cf, net string
		)
		if err := rows.Scan(&it.ID, &it.SettlementID, &it.PLID, &it.Kind, &gross, &pf, &cf, &net,
			&it.VerifiedTxHash, &it.CreatedAt); err != nil {
			return domain.Settlement{}, nil, err
		}
		it.Gross = mustInt(gross)
		it.PlatformFee = mustInt(pf)
		it.ChainFee = mustInt(cf)
		it.Net = mustInt(net)
		items = append(items, it)
	}
	return st, items, rows.Err()
}

// ListSettlements returns a merchant's settlements, optionally filtered by status, newest first.
func (s *Store) ListSettlements(ctx context.Context, merchantKey, status string, limit int) ([]domain.Settlement, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+settlementColumns+` FROM settlement.settlements
		  WHERE merchant_key=$1 AND ($2='' OR status=$2)
		  ORDER BY opened_at DESC, id DESC LIMIT $3`, merchantKey, status, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSettlements(rows)
}

// GetPayout returns a merchant-owned payout.
func (s *Store) GetPayout(ctx context.Context, id, merchantKey string) (domain.Payout, error) {
	p, err := scanPayout(s.pool.QueryRow(ctx,
		`SELECT `+payoutColumns+` FROM settlement.payouts WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payout{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Payout{}, err
	}
	if p.MerchantKey != merchantKey {
		return domain.Payout{}, domain.ErrNotFound
	}
	return p, nil
}

// ListPayouts returns a merchant's payouts, optionally filtered by status, newest first.
func (s *Store) ListPayouts(ctx context.Context, merchantKey, status string, limit int) ([]domain.Payout, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+payoutColumns+` FROM settlement.payouts
		  WHERE merchant_key=$1 AND ($2='' OR status=$2)
		  ORDER BY created_at DESC, id DESC LIMIT $3`, merchantKey, status, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Payout
	for rows.Next() {
		p, err := scanPayout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── shared helpers ───────────────────────────────────────────────────────────────────────────────

// upsertOpenSettlement inserts an OPEN settlement for the period (or returns the existing id).
// isNew reports whether this call created the row (→ publish settlement.batch_created).
func upsertOpenSettlement(ctx context.Context, tx pgx.Tx, merchantKey, currency, date string, cutoff time.Time) (string, bool, error) {
	id := uuid.NewString()
	var got string
	err := tx.QueryRow(ctx,
		`INSERT INTO settlement.settlements (id, merchant_key, currency, settlement_date, status, cutoff_at)
		 VALUES ($1,$2,$3,$4::date,'OPEN',$5)
		 ON CONFLICT (merchant_key, currency, settlement_date) DO NOTHING
		 RETURNING id`, id, merchantKey, currency, date, cutoff).Scan(&got)
	if err == nil {
		return got, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	// Conflict — the period already has a settlement; return its id.
	if err := tx.QueryRow(ctx,
		`SELECT id FROM settlement.settlements WHERE merchant_key=$1 AND currency=$2 AND settlement_date=$3::date`,
		merchantKey, currency, date).Scan(&got); err != nil {
		return "", false, err
	}
	return got, false, nil
}

// bumpSettlement adds the deltas to a settlement's running totals and returns the refreshed row.
func bumpSettlement(ctx context.Context, tx pgx.Tx, id string, dGross, dPlatform, dChain, dNet *big.Int) (domain.Settlement, error) {
	return scanSettlement(tx.QueryRow(ctx,
		`UPDATE settlement.settlements
		    SET gross = gross + $2::numeric, platform_fee = platform_fee + $3::numeric,
		        chain_fee = chain_fee + $4::numeric, net = net + $5::numeric
		  WHERE id = $1
		  RETURNING `+settlementColumns,
		id, dGross.String(), dPlatform.String(), dChain.String(), dNet.String()))
}

const settlementColumns = `id, merchant_key, currency, settlement_date, status, gross::text,
	platform_fee::text, chain_fee::text, net::text, cutoff_at, opened_at, closed_at`

func settlementColumnsPrefixed(p string) string {
	return p + `.id, ` + p + `.merchant_key, ` + p + `.currency, ` + p + `.settlement_date, ` + p + `.status, ` +
		p + `.gross::text, ` + p + `.platform_fee::text, ` + p + `.chain_fee::text, ` + p + `.net::text, ` +
		p + `.cutoff_at, ` + p + `.opened_at, ` + p + `.closed_at`
}

const payoutColumns = `id, settlement_id, merchant_key, rail, currency, amount::text, status,
	reference, scheduled_for, instructed_at, paid_at`

type scannable interface {
	Scan(dest ...any) error
}

func scanSettlement(r scannable) (domain.Settlement, error) {
	var (
		st                 domain.Settlement
		date               time.Time
		gross, pf, cf, net string
		closedAt           *time.Time
	)
	if err := r.Scan(&st.ID, &st.MerchantKey, &st.Currency, &date, &st.Status,
		&gross, &pf, &cf, &net, &st.CutoffAt, &st.OpenedAt, &closedAt); err != nil {
		return domain.Settlement{}, err
	}
	st.SettlementDate = date.Format("2006-01-02")
	st.Gross = mustInt(gross)
	st.PlatformFee = mustInt(pf)
	st.ChainFee = mustInt(cf)
	st.Net = mustInt(net)
	st.ClosedAt = closedAt
	return st, nil
}

func scanSettlements(rows pgx.Rows) ([]domain.Settlement, error) {
	var out []domain.Settlement
	for rows.Next() {
		st, err := scanSettlement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func scanPayout(r scannable) (domain.Payout, error) {
	var (
		p      domain.Payout
		amount string
	)
	if err := r.Scan(&p.ID, &p.SettlementID, &p.MerchantKey, &p.Rail, &p.Currency, &amount,
		&p.Status, &p.Reference, &p.ScheduledFor, &p.InstructedAt, &p.PaidAt); err != nil {
		return domain.Payout{}, err
	}
	p.Amount = mustInt(amount)
	return p, nil
}

func mustInt(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return v
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return 20
	case n > 100:
		return 100
	default:
		return n
	}
}
