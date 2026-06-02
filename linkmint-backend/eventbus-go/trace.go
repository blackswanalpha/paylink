package eventbus

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Bus trace propagation (work18). W3C trace context rides in Kafka record HEADERS, not the Envelope,
// so the byte-identical wire contract is untouched and a trace started in one service (any language)
// continues across the async hop. Everything here is a no-op until a service calls telemetry.Init —
// the global propagator and tracer default to no-ops, so headers stay empty and behavior is unchanged.

func tracer() trace.Tracer { return otel.Tracer("eventbus") }

// injectTraceHeaders serializes the active trace context into Kafka record headers. Returns nil when
// no context is active, leaving the record header-free.
func injectTraceHeaders(ctx context.Context) []kgo.RecordHeader {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return nil
	}
	hs := make([]kgo.RecordHeader, 0, len(carrier))
	for k, v := range carrier {
		hs = append(hs, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return hs
}

// startConsumeSpan extracts the trace context from a record's headers and starts a CONSUMER span as
// the continuation of the producer's trace; the returned context carries it into the handler.
func startConsumeSpan(ctx context.Context, name string, headers []kgo.RecordHeader) (context.Context, trace.Span) {
	carrier := propagation.MapCarrier{}
	for _, h := range headers {
		carrier[h.Key] = string(h.Value)
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	return tracer().Start(ctx, "consume "+name, trace.WithSpanKind(trace.SpanKindConsumer))
}
