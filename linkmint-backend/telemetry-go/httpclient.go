package telemetry

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// roundTripper starts a client span and injects the active W3C trace context into the outbound
// request headers, so a downstream service continues the same trace.
type roundTripper struct{ base http.RoundTripper }

func (rt roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, span := otel.Tracer("telemetry/httpclient").Start(
		r.Context(), "HTTP "+r.Method, trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()
	r = r.Clone(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))
	return rt.base.RoundTrip(r)
}

// WrapTransport wraps an http.RoundTripper so outbound requests carry the current trace context.
// Compose it onto a service's existing client: client.Transport = telemetry.WrapTransport(client.Transport).
// When telemetry is off the active span context is empty, so Inject writes nothing and the request is
// unchanged.
func WrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return roundTripper{base: base}
}

// Client returns an *http.Client whose transport injects trace context on every call.
func Client() *http.Client {
	return &http.Client{Transport: WrapTransport(http.DefaultTransport)}
}
