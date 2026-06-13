// Package domain holds the settlement-service business model and logic: aggregating verified
// PayLinks into per-merchant settlements (gross/fee/net), scheduling T+1 payouts, ingesting rail
// settlement files, and recording every monetary flow as a balanced double-entry posting (A.6).
//
// Non-custodial (A.1): there are no balance columns and no fund movement. A payout is an
// INSTRUCTION to the merchant's external rail; LinkMint only records the flow and emits the event.
package domain

import (
	"errors"
	"math/big"
	"time"
)

// Sentinel errors mapped to the HTTP envelope at the server boundary.
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidState  = errors.New("invalid state")
	ErrInvalidAmount = errors.New("invalid amount")
)

// Settlement status lifecycle: OPEN (accumulating verified PayLinks for the period) →
// CLOSED (cutoff passed; payout schedulable) → PAID (rail file matched the payout).
const (
	StatusOpen   = "OPEN"
	StatusClosed = "CLOSED"
	StatusPaid   = "PAID"
)

// Payout status lifecycle: SCHEDULED → INSTRUCTED (emitted to the rail, A.1) → PAID (rail file
// matched) | FAILED.
const (
	PayoutScheduled  = "SCHEDULED"
	PayoutInstructed = "INSTRUCTED"
	PayoutPaid       = "PAID"
	PayoutFailed     = "FAILED"
)

// Settlement is one merchant's settlement for a (merchant_key, currency, settlement_date) period.
// Amounts are minor units (exact integers); net = gross - platform_fee - chain_fee.
type Settlement struct {
	ID             string
	MerchantKey    string // the PayLink Receiver address — the merchant's on-chain payout identity
	Currency       string
	SettlementDate string // YYYY-MM-DD in the merchant's cutoff timezone
	Status         string
	Gross          *big.Int
	PlatformFee    *big.Int
	ChainFee       *big.Int
	Net            *big.Int
	CutoffAt       time.Time // T+1 instant: a settlement closes when now >= CutoffAt
	OpenedAt       time.Time
	ClosedAt       *time.Time
}

// SettlementItem is one PayLink's contribution to a settlement (one row per pl_id). A clawback is
// recorded as an item with a negative net (Kind == ItemClawback) that offsets the merchant's payout.
type SettlementItem struct {
	ID             string
	SettlementID   string
	PLID           string
	Kind           string // ItemPayLink | ItemClawback
	Gross          *big.Int
	PlatformFee    *big.Int
	ChainFee       *big.Int
	Net            *big.Int
	VerifiedTxHash string
	CreatedAt      time.Time
}

// Item kinds.
const (
	ItemPayLink  = "paylink"
	ItemClawback = "clawback"
)

// Payout is an instruction to pay a merchant's net settlement over their external rail. It holds no
// funds (A.1) — it records the instruction and its lifecycle so a rail settlement file can match it.
type Payout struct {
	ID           string
	SettlementID string
	MerchantKey  string
	Rail         string
	Currency     string
	Amount       *big.Int
	Status       string
	Reference    string // stable matching reference echoed on the rail settlement file
	ScheduledFor time.Time
	InstructedAt *time.Time
	PaidAt       *time.Time
}

// Merchant is the local projection of a merchant from merchant.onboarded (work10), used to enrich
// payout routing where an address↔merchant_id link is available.
type Merchant struct {
	MerchantID  string
	TZ          string
	DefaultRail string
	Status      string
}

// BankAccount is the local projection of a verified merchant bank account from
// merchant.bank_account.verified (work10): rail + currency + status only (no plaintext account ref).
type BankAccount struct {
	BankAccountID string
	MerchantID    string
	Rail          string
	Currency      string
	Status        string
}

// RailFileLine is one parsed line of an ingested rail settlement file.
type RailFileLine struct {
	Reference string   `json:"reference"`
	Amount    *big.Int `json:"-"`
	AmountStr string   `json:"amount"`
	Currency  string   `json:"currency"`
	Status    string   // MATCHED | UNMATCHED — set during ingest
}

// Rail file line statuses.
const (
	LineMatched   = "MATCHED"
	LineUnmatched = "UNMATCHED"
)

// IngestResult summarizes a rail-file ingest: how many lines matched a payout (and were marked PAID)
// versus left for the work27 reconciliation algorithm.
type IngestResult struct {
	FileID    string
	Rail      string
	LineCount int
	Matched   int
	Unmatched int
	// PaidPayouts are the payouts transitioned to PAID by this ingest (for event publishing).
	PaidPayouts []Payout
}
