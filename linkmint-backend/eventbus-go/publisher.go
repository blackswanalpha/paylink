package eventbus

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace"
)

type ctxKey int

const correlationKey ctxKey = 0

// WithCorrelationID returns a context carrying the request correlation id; Publish stamps it into
// the envelope's correlation_id so a trace survives the async hop.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

func correlationFrom(ctx context.Context) string {
	if v, ok := ctx.Value(correlationKey).(string); ok {
		return v
	}
	return ""
}

// Publisher produces domain events to Kafka. Its Publish method matches the services' existing
// domain.Publisher seam — Publish(ctx, name, key string, payload any) error — so it drops in for the
// LogPublisher with no call-site change.
type Publisher struct {
	client *kgo.Client
	source string
	log    *slog.Logger
}

// NewPublisher builds a synchronous, idempotent producer (acks=all). source is the producing service
// name, stamped into every envelope.
func NewPublisher(cfg Config, source string, log *slog.Logger) (*Publisher, error) {
	if log == nil {
		log = slog.Default()
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("eventbus: no kafka brokers configured")
	}
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(0),
	)
	if err != nil {
		return nil, fmt.Errorf("eventbus: new producer: %w", err)
	}
	return &Publisher{client: cl, source: source, log: log}, nil
}

// Publish wraps payload in a canonical Envelope and produces it synchronously to the name's domain
// topic, waiting for the broker ack (at-least-once). The Kafka message key is the entity key, so all
// events for one entity stay ordered on a partition.
func (p *Publisher) Publish(ctx context.Context, name, key string, payload any) error {
	env, err := NewEnvelope(name, key, correlationFrom(ctx), p.source, payload)
	if err != nil {
		return fmt.Errorf("eventbus: build envelope %q: %w", name, err)
	}
	value, err := env.Marshal()
	if err != nil {
		return fmt.Errorf("eventbus: marshal envelope %q: %w", name, err)
	}
	// Start a producer span and stamp the trace context into the record headers (no-op when
	// telemetry is off), so a consumer continues the same trace across the async hop.
	ctx, span := tracer().Start(ctx, "publish "+name, trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	rec := &kgo.Record{Topic: TopicFor(name), Key: []byte(key), Value: value, Headers: injectTraceHeaders(ctx)}
	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("eventbus: produce %q: %w", name, err)
	}
	return nil
}

// Ping checks broker connectivity (for readiness probes).
func (p *Publisher) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close flushes and closes the producer.
func (p *Publisher) Close() { p.client.Close() }
