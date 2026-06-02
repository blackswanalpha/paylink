package eventbus

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestBusTracePropagation verifies the producer injects a W3C traceparent into Kafka record headers
// and the consumer side rebuilds the same trace from them — the cross-service hop, without a broker.
func TestBusTracePropagation(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	headers := injectTraceHeaders(ctx)
	var found bool
	for _, h := range headers {
		if h.Key == "traceparent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("traceparent not injected into headers: %+v", headers)
	}

	consumeCtx, span := startConsumeSpan(context.Background(), "paylink.verified", headers)
	defer span.End()
	if got := trace.SpanContextFromContext(consumeCtx).TraceID(); got != traceID {
		t.Fatalf("consumer trace id %s != producer %s", got, traceID)
	}
}

func TestInjectTraceHeadersNoSpan(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	if injectTraceHeaders(context.Background()) != nil {
		t.Fatal("expected nil headers when no span is active")
	}
}
