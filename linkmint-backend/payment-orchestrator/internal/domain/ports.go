package domain

import (
	"context"
	"errors"
	"time"

	"github.com/paylink/payment-orchestrator/internal/lifecycle"
)

// Store errors.
var (
	// ErrNotFound is returned when no payment matches the lookup.
	ErrNotFound = errors.New("payment not found")
	// ErrPaymentExists is returned by CreatePayment when the paylink already has a payment
	// (one PayLink settles exactly once — invariant A.7).
	ErrPaymentExists = errors.New("payment already exists for paylink")
)

// ProjectFn computes the next lifecycle state from the current one. See lifecycle.Project.
// It returns changed=false for a no-op (idempotent replay) and a non-nil error for an illegal
// transition; the store applies an update only when changed && err == nil.
type ProjectFn func(current lifecycle.State) (next lifecycle.State, changed bool, err error)

// ChainEventRef identifies a single on-chain event for idempotent application + audit. The
// (PayLinkID, Seq) pair is the dedupe key — a redelivered event with the same pair is ignored.
type ChainEventRef struct {
	PayLinkID string
	Seq       uint64
	Kind      string
	TxHash    string
}

// Store persists payments and applies on-chain truth atomically. Implementations: store/memory
// (tests, dev) and store/postgres (production).
type Store interface {
	CreatePayment(ctx context.Context, p Payment) error
	GetPayment(ctx context.Context, id string) (Payment, error)
	GetPaymentByPayLink(ctx context.Context, paylinkID string) (Payment, error)
	// SearchPayments returns payments matching an exact id/paylink_id or a status (read-only admin
	// lookup, most-recent-first). It performs no chain reconcile — it is a cheap projection read.
	SearchPayments(ctx context.Context, q string, limit int) ([]Payment, error)
	// ApplyChainEvent atomically and idempotently advances the payment for ev.PayLinkID toward
	// on-chain truth computed by project. Duplicate (PayLinkID, Seq) refs return changed=false
	// without re-applying. Returns ErrNotFound when no payment exists for the paylink.
	ApplyChainEvent(ctx context.Context, ev ChainEventRef, project ProjectFn) (Payment, bool, error)
	// Reconcile advances the payment toward on-chain truth on the read path (no event dedupe).
	Reconcile(ctx context.Context, paylinkID string, project ProjectFn) (Payment, bool, error)
	Ping(ctx context.Context) error
}

// PayLinkRecord is the subset of a PayLink the orchestrator needs from paylink-service.
type PayLinkRecord struct {
	ID     string
	Status string
	Expiry time.Time
}

// PayLinkLookup reads PayLink records (from paylink-service).
type PayLinkLookup interface {
	// GetPayLink returns the record, or (nil, nil) when it does not exist.
	GetPayLink(ctx context.Context, paylinkID string) (*PayLinkRecord, error)
}

// ChainReader reads authoritative on-chain PayLink status (via the lVM JSON-RPC). Settlement
// truth comes from here (invariant A.3) — the orchestrator never invents it.
type ChainReader interface {
	// PayLinkStatus returns the on-chain status string; found=false if unknown on-chain.
	PayLinkStatus(ctx context.Context, paylinkID string) (status string, found bool, err error)
}

// Publisher emits domain events by logical name (transport seam — Kafka/SQS, ADR-004).
type Publisher interface {
	Publish(ctx context.Context, name, key string, payload any) error
}

// TransitionRecorder is an optional metrics hook for lifecycle transitions (nil-safe).
type TransitionRecorder interface {
	Transition(from, to string)
}
