package domain

import (
	"context"
	"log/slog"
	"math/big"
	"time"
)

// Consumed-event result labels (settlement_events_consumed_total{result}).
const (
	ResultSettled   = "settled"
	ResultFee       = "fee"
	ResultClawback  = "clawback"
	ResultMerchant  = "merchant"
	ResultBank      = "bank"
	ResultDuplicate = "duplicate"
	ResultIgnored   = "ignored"
	ResultError     = "error"
)

// Recorder records metrics (nil-safe — a nil Recorder is a no-op).
type Recorder interface {
	EventConsumed(result string)
	Payout(status string)
	ScheduleTick()
}

// VerifiedEvent is a decoded chain.paylink.verified event (enriched with payee+amount in work23).
type VerifiedEvent struct {
	PLID       string
	Payee      string
	Amount     *big.Int
	TxHash     string
	OccurredAt time.Time
}

// FeeEvent is a decoded chain.fee.collected event (TotalFee is the chain/protocol fee, A.5).
type FeeEvent struct {
	PLID     string
	ChainFee *big.Int
}

// MerchantOnboardedEvent is a decoded merchant.onboarded event (projection enrichment).
type MerchantOnboardedEvent struct {
	MerchantID  string
	TZ          string
	DefaultRail string
	Status      string
}

// BankAccountVerifiedEvent is a decoded merchant.bank_account.verified event.
type BankAccountVerifiedEvent struct {
	BankAccountID string
	MerchantID    string
	Rail          string
	Currency      string
	Status        string
}

// ClawbackEvent is a decoded refund.clawback.requested event.
type ClawbackEvent struct {
	RefundID   string
	PLID       string
	Amount     *big.Int
	OccurredAt time.Time
}

// Service is the settlement domain logic: it owns the period/fee computation and event publishing,
// and delegates the transactional state+ledger writes to the Store.
type Service struct {
	store     Store
	pub       Publisher
	log       *slog.Logger
	m         Recorder
	currency  string
	loc       *time.Location
	feeBps    int64
	minPayout func(string) *big.Int
	rail      string
	now       func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithMetrics sets the metrics recorder.
func WithMetrics(m Recorder) Option { return func(s *Service) { s.m = m } }

// WithCurrency sets the single settlement currency (Phase 2).
func WithCurrency(c string) Option { return func(s *Service) { s.currency = c } }

// WithTimezone sets the cutoff timezone (falls back to UTC if it cannot be loaded).
func WithTimezone(name string) Option {
	return func(s *Service) {
		if loc, err := time.LoadLocation(name); err == nil && loc != nil {
			s.loc = loc
		}
	}
}

// WithPlatformFeeBps sets the optional platform fee in basis points of gross (A.5).
func WithPlatformFeeBps(bps int64) Option { return func(s *Service) { s.feeBps = bps } }

// WithMinPayout sets the per-currency minimum net payout resolver.
func WithMinPayout(fn func(string) *big.Int) Option { return func(s *Service) { s.minPayout = fn } }

// WithDefaultRail sets the fallback payout rail.
func WithDefaultRail(rail string) Option { return func(s *Service) { s.rail = rail } }

// WithClock overrides the clock (tests).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// NewService builds a Service (log may be nil → slog.Default).
func NewService(store Store, pub Publisher, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		store:     store,
		pub:       pub,
		log:       log,
		currency:  "KES",
		loc:       time.UTC,
		minPayout: func(string) *big.Int { return big.NewInt(0) },
		rail:      "mpesa",
		now:       time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// HandleVerified aggregates a verified PayLink into its merchant's OPEN settlement and posts the
// gross ledger entry. Idempotent (DbDedupe on pl_id in the store). Returns a metrics result label.
func (s *Service) HandleVerified(ctx context.Context, ev VerifiedEvent) (string, error) {
	if ev.PLID == "" || ev.Payee == "" || ev.Amount == nil || ev.Amount.Sign() <= 0 {
		s.log.Warn("settlement_verified_incomplete", "pl_id", ev.PLID, "payee", ev.Payee)
		return ResultIgnored, nil
	}
	date, cutoff := s.period(ev.OccurredAt)
	out, err := s.store.RecordVerified(ctx, VerifiedRecord{
		PLID:           ev.PLID,
		MerchantKey:    ev.Payee,
		Currency:       s.currency,
		SettlementDate: date,
		CutoffAt:       cutoff,
		Gross:          ev.Amount,
		PlatformFee:    s.platformFee(ev.Amount),
		TxHash:         ev.TxHash,
	})
	if err != nil {
		return ResultError, err
	}
	if out.Opened {
		s.publish(ctx, EventSettlementBatchCreated, out.Settlement.MerchantKey, map[string]any{
			"settlement_id":   out.Settlement.ID,
			"merchant_key":    out.Settlement.MerchantKey,
			"currency":        out.Settlement.Currency,
			"settlement_date": out.Settlement.SettlementDate,
		})
	}
	if !out.Applied {
		return ResultDuplicate, nil
	}
	return ResultSettled, nil
}

// HandleFee attaches the chain fee for a settled PayLink and posts the fee ledger entry.
func (s *Service) HandleFee(ctx context.Context, ev FeeEvent) (string, error) {
	if ev.PLID == "" || ev.ChainFee == nil || ev.ChainFee.Sign() < 0 {
		s.log.Warn("settlement_fee_incomplete", "pl_id", ev.PLID)
		return ResultIgnored, nil
	}
	out, err := s.store.RecordFee(ctx, FeeRecord{PLID: ev.PLID, ChainFee: ev.ChainFee})
	if err != nil {
		return ResultError, err
	}
	if !out.Found {
		s.log.Warn("settlement_fee_no_item", "pl_id", ev.PLID)
		return ResultIgnored, nil
	}
	if !out.Applied {
		return ResultDuplicate, nil
	}
	return ResultFee, nil
}

// HandleClawback records a refund clawback as a negative offset against the merchant's next OPEN
// settlement (instruction-only; no funds move, A.1).
func (s *Service) HandleClawback(ctx context.Context, ev ClawbackEvent) (string, error) {
	if ev.PLID == "" || ev.Amount == nil || ev.Amount.Sign() <= 0 {
		s.log.Warn("settlement_clawback_incomplete", "refund_id", ev.RefundID, "pl_id", ev.PLID)
		return ResultIgnored, nil
	}
	when := ev.OccurredAt
	if when.IsZero() {
		when = s.now()
	}
	date, cutoff := s.period(when)
	out, err := s.store.RecordClawback(ctx, ClawbackRecord{
		RefundID:       ev.RefundID,
		PLID:           ev.PLID,
		Amount:         ev.Amount,
		SettlementDate: date,
		CutoffAt:       cutoff,
	})
	if err != nil {
		return ResultError, err
	}
	if !out.Found {
		s.log.Warn("settlement_clawback_no_item", "refund_id", ev.RefundID, "pl_id", ev.PLID)
		return ResultIgnored, nil
	}
	if !out.Applied {
		return ResultDuplicate, nil
	}
	return ResultClawback, nil
}

// HandleMerchantOnboarded upserts the merchant projection.
func (s *Service) HandleMerchantOnboarded(ctx context.Context, ev MerchantOnboardedEvent) (string, error) {
	if ev.MerchantID == "" {
		return ResultIgnored, nil
	}
	applied, err := s.store.UpsertMerchant(ctx, Merchant{
		MerchantID: ev.MerchantID, TZ: ev.TZ, DefaultRail: ev.DefaultRail, Status: ev.Status,
	})
	if err != nil {
		return ResultError, err
	}
	if !applied {
		return ResultDuplicate, nil
	}
	return ResultMerchant, nil
}

// HandleBankAccountVerified upserts the bank-account projection.
func (s *Service) HandleBankAccountVerified(ctx context.Context, ev BankAccountVerifiedEvent) (string, error) {
	if ev.BankAccountID == "" {
		return ResultIgnored, nil
	}
	applied, err := s.store.UpsertBankAccount(ctx, BankAccount{
		BankAccountID: ev.BankAccountID, MerchantID: ev.MerchantID,
		Rail: ev.Rail, Currency: ev.Currency, Status: ev.Status,
	})
	if err != nil {
		return ResultError, err
	}
	if !applied {
		return ResultDuplicate, nil
	}
	return ResultBank, nil
}

// Schedule runs one payout-scheduling pass: close due settlements (T+1 cutoff), then create+instruct
// a payout for each that meets the minimum. Errors are logged; the loop never dies (mirrors escrow).
func (s *Service) Schedule(ctx context.Context) {
	if _, err := s.store.CloseDueSettlements(ctx, s.now()); err != nil {
		s.log.Error("settlement_close_due_failed", "err", err.Error())
	}
	payouts, err := s.store.SchedulePayouts(ctx, s.now(), ScheduleOpts{
		MinPayoutFor: s.minPayout, DefaultRail: s.rail,
	})
	if err != nil {
		s.log.Error("settlement_schedule_payouts_failed", "err", err.Error())
		return
	}
	for _, p := range payouts {
		s.emitPayout(ctx, p)
	}
}

// CreatePayout creates an on-demand payout for a CLOSED, merchant-owned settlement.
func (s *Service) CreatePayout(ctx context.Context, settlementID, merchantKey string) (Payout, error) {
	p, err := s.store.CreatePayout(ctx, settlementID, merchantKey, s.rail)
	if err != nil {
		return Payout{}, err
	}
	s.emitPayout(ctx, p)
	return p, nil
}

// IngestRailFile records a rail settlement file, matches its lines to payouts, and publishes a
// payout.completed + settlement.completed for each newly-PAID payout.
func (s *Service) IngestRailFile(ctx context.Context, in RailFileInput) (IngestResult, error) {
	res, err := s.store.IngestRailFile(ctx, in)
	if err != nil {
		return IngestResult{}, err
	}
	for _, p := range res.PaidPayouts {
		if s.m != nil {
			s.m.Payout(PayoutPaid)
		}
		s.publish(ctx, EventPayoutCompleted, p.MerchantKey, payoutPayload(p))
		s.publish(ctx, EventSettlementCompleted, p.MerchantKey, map[string]any{
			"settlement_id": p.SettlementID,
			"payout_id":     p.ID,
			"merchant_key":  p.MerchantKey,
			"currency":      p.Currency,
			"amount":        p.Amount.String(),
		})
	}
	return res, nil
}

// GetSettlement returns a merchant-owned settlement with its items.
func (s *Service) GetSettlement(ctx context.Context, id, merchantKey string) (Settlement, []SettlementItem, error) {
	return s.store.GetSettlement(ctx, id, merchantKey)
}

// ListSettlements returns a merchant's settlements, optionally filtered by status.
func (s *Service) ListSettlements(ctx context.Context, merchantKey, status string, limit int) ([]Settlement, error) {
	return s.store.ListSettlements(ctx, merchantKey, status, limit)
}

// GetPayout returns a merchant-owned payout.
func (s *Service) GetPayout(ctx context.Context, id, merchantKey string) (Payout, error) {
	return s.store.GetPayout(ctx, id, merchantKey)
}

// ListPayouts returns a merchant's payouts, optionally filtered by status.
func (s *Service) ListPayouts(ctx context.Context, merchantKey, status string, limit int) ([]Payout, error) {
	return s.store.ListPayouts(ctx, merchantKey, status, limit)
}

// emitPayout records metrics and publishes the scheduled (+ instructed) events for a payout.
func (s *Service) emitPayout(ctx context.Context, p Payout) {
	if s.m != nil {
		s.m.Payout(PayoutScheduled)
	}
	s.publish(ctx, EventPayoutScheduled, p.MerchantKey, payoutPayload(p))
	if p.Status == PayoutInstructed {
		if s.m != nil {
			s.m.Payout(PayoutInstructed)
		}
		s.publish(ctx, EventPayoutInstructed, p.MerchantKey, payoutPayload(p))
	}
}

// publish fire-and-logs a domain event. A publish failure is logged, not fatal: the state write has
// already committed, and DbDedupe would suppress a re-publish on redelivery (at-most-once publish —
// a documented MVP gap, same as escrow-manager; the durable fix is an outbox relay, work15).
func (s *Service) publish(ctx context.Context, name, key string, payload any) {
	if s.pub == nil {
		return
	}
	if err := s.pub.Publish(ctx, name, key, payload); err != nil {
		s.log.Warn("settlement_publish_failed", "event", name, "key", key, "err", err.Error())
	}
}

// period returns the settlement date (YYYY-MM-DD) and the T+1 cutoff instant for a verified time,
// computed in the service's configured timezone.
func (s *Service) period(ts time.Time) (string, time.Time) {
	if ts.IsZero() {
		ts = s.now()
	}
	t := ts.In(s.loc)
	y, m, d := t.Date()
	cutoff := time.Date(y, m, d, 0, 0, 0, 0, s.loc).AddDate(0, 0, 1)
	return t.Format("2006-01-02"), cutoff
}

// platformFee returns floor(gross * feeBps / 10000) (A.5 — separate from the chain fee).
func (s *Service) platformFee(gross *big.Int) *big.Int {
	if s.feeBps <= 0 || gross == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(gross, big.NewInt(s.feeBps)), big.NewInt(10_000))
}

func payoutPayload(p Payout) map[string]any {
	return map[string]any{
		"payout_id":     p.ID,
		"settlement_id": p.SettlementID,
		"merchant_key":  p.MerchantKey,
		"rail":          p.Rail,
		"currency":      p.Currency,
		"amount":        p.Amount.String(),
		"reference":     p.Reference,
		"status":        p.Status,
	}
}
