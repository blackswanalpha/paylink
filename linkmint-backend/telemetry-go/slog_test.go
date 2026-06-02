package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestLogHandlerAddsIDs(t *testing.T) {
	withRecording(t)
	var buf bytes.Buffer
	logger := slog.New(NewLogHandler(slog.NewJSONHandler(&buf, nil)).WithAttrs([]slog.Attr{slog.String("service", "svc")}))
	ctx, span := otel.Tracer("t").Start(context.Background(), "op")
	logger.InfoContext(ctx, "hello")
	span.End()
	out := buf.String()
	if !strings.Contains(out, "trace_id") || !strings.Contains(out, "span_id") {
		t.Fatalf("missing trace/span ids: %s", out)
	}
}

func TestLogHandlerNoSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewLogHandler(slog.NewJSONHandler(&buf, nil)).WithGroup("g"))
	logger.Info("x", "k", "v")
	if strings.Contains(buf.String(), "trace_id") {
		t.Fatalf("should not add ids without an active span: %s", buf.String())
	}
}
