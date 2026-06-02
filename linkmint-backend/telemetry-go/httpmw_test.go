package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// withRecording installs a real (always-sampling) provider + the W3C propagator so spans have valid
// trace ids in-process. Shared by the middleware/client/slog tests.
func withRecording(t *testing.T) {
	t.Helper()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
}

func TestMiddlewareSeedsTraceID(t *testing.T) {
	withRecording(t)
	var seenReqID, seenTrace string
	h := Middleware("svc", func(r *http.Request) string { return "/v1/things/{id}" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenReqID = r.Header.Get("X-Request-Id")
			seenTrace = trace.SpanContextFromContext(r.Context()).TraceID().String()
			w.WriteHeader(http.StatusCreated)
		}),
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/things/abc", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}
	if seenReqID == "" || seenReqID != seenTrace {
		t.Fatalf("X-Request-Id %q should equal trace id %q", seenReqID, seenTrace)
	}
}

func TestMiddlewareNilRouteOf(t *testing.T) {
	withRecording(t)
	h := Middleware("svc", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", nil))
}

func TestHTTPRoundTripPropagation(t *testing.T) {
	withRecording(t)
	var serverTrace string
	srv := httptest.NewServer(Middleware("downstream", func(r *http.Request) string { return "/v1/echo" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverTrace = trace.SpanContextFromContext(r.Context()).TraceID().String()
			w.WriteHeader(http.StatusOK)
		}),
	))
	defer srv.Close()

	ctx, span := otel.Tracer("test").Start(context.Background(), "root")
	rootTrace := span.SpanContext().TraceID().String()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	resp, err := Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	span.End()

	if serverTrace == "" || serverTrace != rootTrace {
		t.Fatalf("trace not propagated: server=%s root=%s", serverTrace, rootTrace)
	}
}

func TestWrapTransportNilBase(t *testing.T) {
	if WrapTransport(nil) == nil {
		t.Fatal("WrapTransport(nil) should default the base transport")
	}
}
