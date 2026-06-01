// Package intake is the audit.intake consumer seam. The event bus (NATS) is work15 and is not
// built yet, so the live Phase-1 intake path is the HTTP POST /v1/audit-log. This package defines
// the consumer interface and a no-op source; when work15 lands, a NATS-backed Source subscribes to
// the audit.intake subject and calls Handler (which wraps domain.Service.Append) — no other change.
package intake

import (
	"context"
	"log/slog"

	"github.com/paylink/audit-log-service/internal/domain"
)

// Handler appends one intake message to the chain (wraps domain.Service.Append).
type Handler func(ctx context.Context, in domain.AppendInput) error

// Source delivers audit.intake messages to a Handler until ctx is cancelled.
type Source interface {
	Run(ctx context.Context, h Handler) error
}

// NoopSource is the work15 placeholder: it blocks until ctx is cancelled and delivers nothing.
type NoopSource struct {
	Log *slog.Logger
}

// Run blocks until ctx is done.
func (n NoopSource) Run(ctx context.Context, _ Handler) error {
	if n.Log != nil {
		n.Log.Info("audit intake consumer is a no-op stub (NATS audit.intake lands with work15); the HTTP POST /v1/audit-log is the live intake path")
	}
	<-ctx.Done()
	return ctx.Err()
}
