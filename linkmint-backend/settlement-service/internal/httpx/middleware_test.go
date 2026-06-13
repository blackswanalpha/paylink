package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDPropagatesAndGenerates(t *testing.T) {
	var seenTrace string
	h := RequestID(slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTrace = TraceIDFromContext(r.Context())
		LoggerFromContext(r.Context()).Info("hi")
		w.WriteHeader(http.StatusOK)
	}))

	// Provided id is echoed.
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(RequestIDHeader, "trace-123")
	h.ServeHTTP(rr, r)
	if seenTrace != "trace-123" || rr.Header().Get(RequestIDHeader) != "trace-123" {
		t.Fatalf("trace=%q header=%q, want trace-123", seenTrace, rr.Header().Get(RequestIDHeader))
	}

	// Missing id is generated.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Header().Get(RequestIDHeader) == "" {
		t.Fatal("expected a generated request id")
	}
}

func TestRequestLogger(t *testing.T) {
	h := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestRecovererTurnsPanicInto500(t *testing.T) {
	h := Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
}

func TestContextDefaults(t *testing.T) {
	if TraceIDFromContext(context.Background()) != "" {
		t.Error("missing trace should be empty")
	}
	if LoggerFromContext(context.Background()) == nil {
		t.Error("logger should fall back to default")
	}
}
