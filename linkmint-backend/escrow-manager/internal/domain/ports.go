package domain

import (
	"context"
	"errors"
	"time"

	"github.com/paylink/escrow-manager/internal/fsm"
)

// Store errors.
var (
	// ErrNotFound is returned when no escrow matches the lookup.
	ErrNotFound = errors.New("escrow not found")
	// ErrEscrowExists is returned by CreateEscrow when the paylink already has an escrow
	// (pl_id is UNIQUE — one escrow per PayLink).
	ErrEscrowExists = errors.New("escrow already exists for paylink")
)

// Update describes the atomic changes a MutateFn asks the store to apply to a locked escrow
// row. Zero-valued fields are no-ops; everything set is applied in the SAME transaction.
type Update struct {
	AddApproval   string    // approver address to record ("" = none); idempotent (PK)
	SetFunded     bool      // mark funded (never unset)
	FundedTxHash  string    // chain tx hash recorded with the funding mark
	SetState      fsm.State // new state ("" = no change)
	DisputeReason string    // recorded with a Dispute transition
}

// MutateFn decides the update for an escrow given its current row and the recorded approval
// addresses. It runs exactly once, inside the store's transaction, with the row locked.
// Returning an error aborts the transaction (nothing is applied).
type MutateFn func(e Escrow, approvals []string) (Update, error)

// Store persists escrows and applies condition/funding decisions atomically. Implementations:
// store/memory (tests, dev) and store/postgres (production).
type Store interface {
	CreateEscrow(ctx context.Context, e Escrow) error
	GetEscrow(ctx context.Context, id string) (Escrow, error)
	// ListEscrows returns the creator's escrows, optionally filtered by state, most recent first.
	ListEscrows(ctx context.Context, creatorAddr, state string, limit int) ([]Escrow, error)
	// Mutate locks the escrow by id, calls fn with the row + approvals, and applies the returned
	// Update atomically. Returns the post-update escrow, or ErrNotFound.
	Mutate(ctx context.Context, id string, fn MutateFn) (Escrow, error)
	// ApplyFunding is Mutate keyed by pl_id, deduplicated on (scope="chain.paylink.verified",
	// key=plID+":"+txHash) via a processed_events row written in the SAME transaction as fn's
	// update (work17 DbDedupe) — an at-least-once redelivery applies its effect exactly once.
	// Returns applied=false (and no changes) when the event was already processed.
	ApplyFunding(ctx context.Context, plID, txHash string, fn MutateFn) (e Escrow, applied bool, err error)
	// ReleaseDueTimeLocks CAS-releases funded time_lock escrows whose release_at has passed
	// (UPDATE … WHERE state='WAITING'), returning the rows it transitioned.
	ReleaseDueTimeLocks(ctx context.Context, now time.Time) ([]Escrow, error)
	// RefundTimedOut CAS-refunds escrows whose timeout_at has passed (state WAITING only —
	// DISPUTED rows are never touched), returning the rows it transitioned.
	RefundTimedOut(ctx context.Context, now time.Time) ([]Escrow, error)
	Ping(ctx context.Context) error
}

// Publisher emits domain events by logical name (transport seam — Kafka/SQS, ADR-004).
// Events are published AFTER the owning transaction commits — at-most-once: a crash in
// the commit→publish window drops the event (no outbox; the work15 Go transactional-outbox
// follow-up covers this).
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}

// TransitionRecorder is an optional metrics hook for FSM transitions (nil-safe).
type TransitionRecorder interface {
	Transition(kind string)
}
