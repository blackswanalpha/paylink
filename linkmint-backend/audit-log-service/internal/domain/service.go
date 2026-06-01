package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/paylink/audit-log-service/internal/httpx"
)

// Event names (audit.*) produced by the service. Per spec §2.17.
const (
	EventEntryAdded         = "audit.entry.added"
	EventVerificationFailed = "audit.verification.failed"
)

// Service is the audit-log core logic. All dependencies are injected.
type Service struct {
	store   Store
	pub     Publisher
	metrics Metrics
	log     *slog.Logger
	now     func() time.Time
}

// Metrics is an optional hook (nil-safe via the noopMetrics default).
type Metrics interface {
	AuditEntry(actorKind string)
	Verify(result string)
}

type noopMetrics struct{}

func (noopMetrics) AuditEntry(string) {}
func (noopMetrics) Verify(string)     {}

// Option configures a Service.
type Option func(*Service)

// WithMetrics injects a metrics recorder.
func WithMetrics(m Metrics) Option { return func(s *Service) { s.metrics = m } }

// WithClock overrides the clock (tests).
func WithClock(c func() time.Time) Option { return func(s *Service) { s.now = c } }

// NewService builds a Service.
func NewService(store Store, pub Publisher, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		store:   store,
		pub:     pub,
		metrics: noopMetrics{},
		log:     log,
		now:     func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Append validates the input, stamps occurred_at when absent, appends to the chain, and emits
// audit.entry.added. The entry is hashed and linked inside the store under a serialization lock.
func (s *Service) Append(ctx context.Context, in AppendInput) (Entry, error) {
	if err := in.Validate(); err != nil {
		return Entry{}, err
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = s.now()
	}
	// Normalize to the precision Postgres timestamptz round-trips, so the write-time hash equals the
	// read-time recompute.
	in.OccurredAt = in.OccurredAt.UTC().Truncate(time.Microsecond)

	e, err := s.store.Append(ctx, in)
	if err != nil {
		return Entry{}, err
	}
	s.metrics.AuditEntry(string(e.Actor.Kind))
	// Event payload carries identifiers only (no before/after bodies) — the log is the system of
	// record; the event is a notification.
	_ = s.pub.Publish(ctx, EventEntryAdded, e.Resource, map[string]any{
		"entry_id":    e.EntryID,
		"action":      e.Action,
		"resource":    e.Resource,
		"actor_kind":  string(e.Actor.Kind),
		"occurred_at": e.OccurredAt.Format(time.RFC3339Nano),
		"entry_hash":  hex.EncodeToString(e.EntryHash),
	})
	return e, nil
}

// Get returns the entry and its inclusion proof, or a 404 AppError.
func (s *Service) Get(ctx context.Context, id int64) (Entry, Proof, error) {
	e, err := s.store.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Entry{}, Proof{}, httpx.NewError(httpx.CodeEntryNotFound, "audit entry not found", nil)
	}
	if err != nil {
		return Entry{}, Proof{}, err
	}
	return e, BuildProof(e), nil
}

// Query returns a newest-first page of entries matching the filter.
func (s *Service) Query(ctx context.Context, f QueryFilter) (Page, error) {
	return s.store.Query(ctx, f)
}

// Verify walks the chain over [from,to] (full chain when both nil) and reports the first break.
// A break emits audit.verification.failed — the spec names no success event.
func (s *Service) Verify(ctx context.Context, from, to *time.Time) (VerifyResult, error) {
	res, err := s.store.VerifyRange(ctx, from, to)
	if err != nil {
		return VerifyResult{}, err
	}
	if res.OK {
		s.metrics.Verify("ok")
		return res, nil
	}
	s.metrics.Verify("broken")
	s.log.Error("audit_chain_broken", "broken_at", *res.BrokenAt)
	_ = s.pub.Publish(ctx, EventVerificationFailed, "", map[string]any{
		"broken_at": *res.BrokenAt,
	})
	return res, nil
}
