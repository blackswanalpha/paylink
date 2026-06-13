package domain

import (
	"context"
	"math/big"
	"time"
)

// Publisher publishes a domain event by logical name + partition key. The signature matches
// eventbus-go's Publisher.Publish exactly, so the Kafka publisher drops in unchanged (work15).
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}

// Published event logical names (catalog.md row "settlement.* / payout.*").
const (
	EventSettlementBatchCreated = "settlement.batch_created"
	EventSettlementCompleted    = "settlement.completed"
	EventPayoutScheduled        = "payout.scheduled"
	EventPayoutInstructed       = "payout.instructed"
	EventPayoutCompleted        = "payout.completed"
)

// VerifiedRecord is the persisted effect of a chain.paylink.verified event. The service computes the
// settlement period (date + T+1 cutoff in the merchant's tz) and the platform fee (A.5) before
// calling the store; the store upserts the OPEN settlement, inserts the item, and posts the gross
// ledger entry — all on one transaction, guarded by DbDedupe on the pl_id (exactly-once, work17).
type VerifiedRecord struct {
	PLID           string
	MerchantKey    string
	Currency       string
	SettlementDate string
	CutoffAt       time.Time
	Gross          *big.Int
	PlatformFee    *big.Int
	TxHash         string
}

// VerifiedOutcome reports what RecordVerified did: whether the item was applied (false = duplicate)
// and whether a new settlement was opened (→ publish settlement.batch_created).
type VerifiedOutcome struct {
	Applied    bool
	Opened     bool
	Settlement Settlement
}

// FeeRecord is the persisted effect of a chain.fee.collected event: the chain/protocol fee for a
// settled PayLink (A.5). The store attaches it to the existing item and posts the fee ledger entry.
type FeeRecord struct {
	PLID     string
	ChainFee *big.Int
}

// FeeOutcome reports what RecordFee did. Found=false means no settled item exists for the pl_id yet
// (anomalous — verified always precedes fee on the same partition); the caller acks and logs.
type FeeOutcome struct {
	Applied bool
	Found   bool
}

// ClawbackRecord is the persisted effect of a refund.clawback.requested event: a negative offset
// against the merchant's next OPEN settlement. The store resolves the merchant from the pl_id's
// settled item; SettlementDate/CutoffAt scope the OPEN settlement to attach the offset to.
type ClawbackRecord struct {
	RefundID       string
	PLID           string
	Amount         *big.Int
	SettlementDate string
	CutoffAt       time.Time
}

// ClawbackOutcome reports what RecordClawback did. Found=false means the clawed-back pl_id was never
// settled here (nothing to offset); the caller acks and logs.
type ClawbackOutcome struct {
	Applied bool
	Found   bool
}

// ScheduleOpts parameterizes the payout scheduling pass.
type ScheduleOpts struct {
	// MinPayoutFor returns the minimum net (minor units) a settlement must reach to be paid out in
	// the given currency. Below it, the settlement stays CLOSED with no payout (carried, not paid).
	MinPayoutFor func(currency string) *big.Int
	// DefaultRail is the payout rail used when the merchant's rail cannot be resolved from the
	// bank-account projection (the address↔merchant_id link is a documented seam).
	DefaultRail string
}

// RailFileInput is a parsed rail settlement file submitted to the internal ingest endpoint.
type RailFileInput struct {
	Rail   string
	FileID string
	Lines  []RailFileLine
}

// Store is the persistence + ledger surface. The postgres implementation runs each mutating method
// in one transaction that joins the business-state write, the DbDedupe mark, and the balanced
// ledger.Post (A.6) so they commit or roll back together. The memory implementation mirrors the
// state bookkeeping for unit tests (no ledger).
type Store interface {
	RecordVerified(ctx context.Context, in VerifiedRecord) (VerifiedOutcome, error)
	RecordFee(ctx context.Context, in FeeRecord) (FeeOutcome, error)
	RecordClawback(ctx context.Context, in ClawbackRecord) (ClawbackOutcome, error)

	UpsertMerchant(ctx context.Context, m Merchant) (bool, error)
	UpsertBankAccount(ctx context.Context, b BankAccount) (bool, error)

	// CloseDueSettlements CAS-transitions OPEN settlements whose cutoff has passed to CLOSED and
	// returns them (for any close-time bookkeeping/metrics).
	CloseDueSettlements(ctx context.Context, now time.Time) ([]Settlement, error)
	// SchedulePayouts creates+instructs a payout for each CLOSED settlement that has no payout yet
	// and meets the minimum, returning the affected payouts (status INSTRUCTED or SCHEDULED).
	SchedulePayouts(ctx context.Context, now time.Time, opts ScheduleOpts) ([]Payout, error)

	// CreatePayout creates+instructs an on-demand payout for a CLOSED, merchant-owned settlement
	// that has no payout yet.
	CreatePayout(ctx context.Context, settlementID, merchantKey, defaultRail string) (Payout, error)

	// IngestRailFile records the file + lines, matches each line to a payout by reference (+amount),
	// marks matched payouts PAID and their settlements PAID (posting the payout ledger entry), and
	// leaves unmatched lines for work27 reconciliation.
	IngestRailFile(ctx context.Context, in RailFileInput) (IngestResult, error)

	GetSettlement(ctx context.Context, id, merchantKey string) (Settlement, []SettlementItem, error)
	ListSettlements(ctx context.Context, merchantKey, status string, limit int) ([]Settlement, error)
	GetPayout(ctx context.Context, id, merchantKey string) (Payout, error)
	ListPayouts(ctx context.Context, merchantKey, status string, limit int) ([]Payout, error)

	Ping(ctx context.Context) error
	Close()
}
