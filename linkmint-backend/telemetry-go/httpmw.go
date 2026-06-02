package telemetry

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-Id"

// Middleware extracts W3C trace context from the inbound request, starts a server span, and makes the
// active trace id the request's correlation id by seeding X-Request-Id with the 32-hex trace id. Place
// it BEFORE the service's existing RequestID middleware in the chi chain so that middleware adopts the
// trace id — then slog `trace_id`, the error envelope, the response header and the Tempo trace all
// share ONE id.
//
// routeOf returns the matched route TEMPLATE for a request (pass the service's existing route func, the
// same one given to metrics.Middleware). The template — not the raw path — is used for the span name
// and the http.route attribute, keeping spans low-cardinality and PII-free. When telemetry is off the
// tracer is a no-op: the span context is invalid, so X-Request-Id is left untouched and behavior is
// unchanged.
func Middleware(serviceName string, routeOf func(*http.Request) string) func(http.Handler) http.Handler {
	if routeOf == nil {
		routeOf = func(r *http.Request) string { return r.Method }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracer := otel.Tracer("telemetry/http")
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String("service.name", serviceName)),
			)
			if sc := span.SpanContext(); sc.HasTraceID() {
				r.Header.Set(requestIDHeader, sc.TraceID().String())
			}
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			r = r.WithContext(ctx)
			next.ServeHTTP(rec, r)

			// The chi route pattern is only known after routing, so name the span and record the
			// route here (post-handler) using the template — never the raw URL path.
			route := routeOf(r)
			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.response.status_code", rec.status),
			)
			if rec.status >= 500 {
				span.SetAttributes(attribute.Bool("error", true))
			}
			span.End()
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.wrote = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}
