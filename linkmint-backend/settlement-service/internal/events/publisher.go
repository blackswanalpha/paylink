// Package events is the domain-event publisher seam (domain.Publisher). The real transport is
// the work15 Kafka bus (eventbus-go — its Publish matches domain.Publisher exactly, so it drops
// in unchanged when SETTLEMENT_EVENT_PUBLISHER_MODE=kafka); LogPublisher records publish intent to
// the structured log so the event contract is exercised without a broker.
package events

import (
	"context"
	"log/slog"
)

// LogPublisher writes domain events to the structured log.
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
