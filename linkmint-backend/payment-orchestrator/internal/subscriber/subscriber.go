// Package subscriber consumes lVM chain events from a chain.EventSource and drives the payment
// lifecycle via domain.Service. It is the event-driven fast path; domain.Service.Get provides the
// read-path reconciliation safety net for any events missed during a reconnect gap.
package subscriber

import (
	"context"
	"errors"
	"log/slog"

	"github.com/paylink/payment-orchestrator/internal/chain"
	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/metrics"
)

// Subscriber bridges a chain.EventSource to the domain service.
type Subscriber struct {
	source  chain.EventSource
	svc     *domain.Service
	metrics *metrics.Metrics
	log     *slog.Logger
}

// New builds a Subscriber (log may be nil → slog.Default).
func New(source chain.EventSource, svc *domain.Service, m *metrics.Metrics, log *slog.Logger) *Subscriber {
	if log == nil {
		log = slog.Default()
	}
	return &Subscriber{source: source, svc: svc, metrics: m, log: log}
}

// Run consumes events until ctx is cancelled (or the source returns a fatal error).
func (s *Subscriber) Run(ctx context.Context) error {
	s.log.Info("chain_subscriber_starting")
	return s.source.Run(ctx, s.handle)
}

// handle applies a single chain event. It NEVER returns a non-nil error: a bad or unexpected
// event is logged/metered but must not drop the WebSocket connection.
func (s *Subscriber) handle(ctx context.Context, ev chain.Event) error {
	if ev.EntityType != chain.EntityPayLink || ev.EntityID == "" {
		return nil
	}
	status, ok := chain.ChainStatusForEvent(ev)
	if !ok {
		s.record(ev.Kind, "ignored")
		return nil
	}

	_, changed, err := s.svc.ApplyChainEvent(ctx, domain.ChainEventInput{
		PayLinkID:   ev.EntityID,
		Seq:         ev.Sequence,
		ChainStatus: status,
		Kind:        ev.Kind,
		TxHash:      ev.TxHash,
	})
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// Event for a PayLink this service is not orchestrating — expected, ignore.
		s.record(ev.Kind, "ignored")
	case err != nil:
		s.record(ev.Kind, "error")
		s.log.Error("chain_event_apply_failed", "paylink_id", ev.EntityID, "kind", ev.Kind, "seq", ev.Sequence, "err", err.Error())
	case changed:
		s.record(ev.Kind, "applied")
	default:
		s.record(ev.Kind, "duplicate")
	}
	return nil
}

func (s *Subscriber) record(kind, result string) {
	if s.metrics != nil {
		s.metrics.ChainEvent(kind, result)
	}
}
