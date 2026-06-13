package httpx

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testLogger is a discarding logger used to drive the request middleware in tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestStatusFor(t *testing.T) {
	cases := map[ErrorCode]int{
		CodeWalletNotFound:   http.StatusNotFound,
		CodePositionNotFound: http.StatusNotFound,
		CodeInvalidAddress:   http.StatusBadRequest,
		CodeInvalidAmount:    http.StatusBadRequest,
		CodeUnauthorized:     http.StatusUnauthorized,
		CodeForbidden:        http.StatusForbidden,
		CodeChainUnavailable: http.StatusServiceUnavailable,
		CodeInternalError:    http.StatusInternalServerError,
		ErrorCode("UNKNOWN"): http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := StatusFor(code); got != want {
			t.Errorf("StatusFor(%s) = %d, want %d", code, got, want)
		}
	}
}

func TestAppErrorStatusOverride(t *testing.T) {
	e := &AppError{Code: CodeWalletNotFound, Message: "x", HTTPStatus: http.StatusTeapot}
	if e.Status() != http.StatusTeapot {
		t.Errorf("override status = %d", e.Status())
	}
	if !strings.Contains(e.Error(), "WALLET_NOT_FOUND") {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestAsAppError(t *testing.T) {
	if AsAppError(nil) != nil {
		t.Error("nil should map to nil")
	}
	wrapped := AsAppError(context.DeadlineExceeded)
	if wrapped.Code != CodeInternalError {
		t.Errorf("opaque error code = %s", wrapped.Code)
	}
	orig := NewError(CodeInvalidAmount, "bad", nil)
	if AsAppError(orig) != orig {
		t.Error("AppError should pass through")
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	WriteError(rr, req, NewError(CodeInvalidAddress, "bad addr", map[string]any{"addr": "x"}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	var body envelopeBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "INVALID_ADDRESS" || body.Error.Message != "bad addr" {
		t.Errorf("envelope = %+v", body.Error)
	}
	if body.Error.Details["addr"] != "x" {
		t.Errorf("details = %v", body.Error.Details)
	}
}

func TestDecodeJSON(t *testing.T) {
	type body struct {
		A string `json:"a"`
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"ok"}`))
	var v body
	if err := DecodeJSON(rr, req, &v); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if v.A != "ok" {
		t.Errorf("A = %q", v.A)
	}

	// Unknown field is rejected.
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"ok","b":1}`))
	if err := DecodeJSON(rr, req, &v); err == nil {
		t.Error("expected error for unknown field")
	}

	// Trailing data is rejected.
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"ok"}{}`))
	if err := DecodeJSON(rr, req, &v); err == nil {
		t.Error("expected error for trailing data")
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	var seenTrace string
	h := RequestID(testLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTrace = TraceIDFromContext(r.Context())
		LoggerFromContext(r.Context()).Info("ok")
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "trace-123")
	h.ServeHTTP(rr, req)
	if seenTrace != "trace-123" {
		t.Errorf("trace propagation = %q", seenTrace)
	}
	if rr.Header().Get(RequestIDHeader) != "trace-123" {
		t.Errorf("echoed header = %q", rr.Header().Get(RequestIDHeader))
	}

	// Generates one when absent.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Header().Get(RequestIDHeader) == "" {
		t.Error("expected a generated request id")
	}
}

func TestRecoverer(t *testing.T) {
	h := RequestID(testLogger())(RequestLogger(Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestTraceIDEmptyContext(t *testing.T) {
	if TraceIDFromContext(context.Background()) != "" {
		t.Error("expected empty trace id")
	}
	if LoggerFromContext(context.Background()) == nil {
		t.Error("expected default logger")
	}
}
