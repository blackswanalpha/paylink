// Package events is the domain-event publisher seam (domain.Publisher). The real transport
// (Kafka/SQS/NATS, ADR-004) is deferred to work15; today LogPublisher records publish intent to
// the structured log so the audit.* event contract (audit.entry.added, audit.verification.failed)
// is exercised end-to-end without coupling to a broker.
package events

import (
	"context"
	"log/slog"
)

// LogPublisher writes domain events to the structured log. It is the work15 transport seam.
type LogPublisher struct {
	log *slog.Logger
}

// NewLogPublisher builds a LogPublisher (log may be nil → slog.Default).
func NewLogPublisher(log *slog.Logger) *LogPublisher {
	if log == nil {
		log = slog.Default()
	}
	return &LogPublisher{log: log}
}

// Publish records the event. It never fails (the seam is fire-and-log).
func (p *LogPublisher) Publish(_ context.Context, name, key string, payload any) error {
	p.log.Info("domain_event_published", "event", name, "key", key, "payload", payload)
	return nil
}
