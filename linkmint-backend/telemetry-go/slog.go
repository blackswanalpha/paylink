package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// LogHandler wraps an slog.Handler so every record logged with a span-carrying context gains
// trace_id/span_id attributes — correlating a service's structured logs with its traces in Tempo.
// Compose it around logging.New's JSON handler. When no span is active (telemetry off) it adds
// nothing, so logs are unchanged.
type LogHandler struct{ slog.Handler }

// NewLogHandler returns h wrapped so it stamps trace_id/span_id from the record's context.
func NewLogHandler(h slog.Handler) slog.Handler { return &LogHandler{Handler: h} }

func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *LogHandler) WithGroup(name string) slog.Handler {
	return &LogHandler{Handler: h.Handler.WithGroup(name)}
}
