package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDGenerated(t *testing.T) {
	var trace string
	h := RequestID(slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace = TraceIDFromContext(r.Context())
		if LoggerFromContext(r.Context()) == nil {
			t.Error("logger should be in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if trace == "" || rr.Header().Get(RequestIDHeader) == "" {
		t.Fatalf("trace=%q header=%q", trace, rr.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDPropagated(t *testing.T) {
	var trace string
	h := RequestID(slog.Default())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		trace = TraceIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "abc-123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if trace != "abc-123" || rr.Header().Get(RequestIDHeader) != "abc-123" {
		t.Fatalf("trace=%q header=%q", trace, rr.Header().Get(RequestIDHeader))
	}
}

func TestRecoverer(t *testing.T) {
	h := RequestID(slog.Default())(Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), string(CodeInternalError)) {
		t.Fatalf("body missing INTERNAL_ERROR: %s", rr.Body)
	}
}

func TestRequestLoggerPassesThrough(t *testing.T) {
	h := RequestID(slog.Default())(RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestWriteErrorAppError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteError(rr, httptest.NewRequest(http.MethodGet, "/", nil),
		NewError(CodePayLinkNotFound, "nope", map[string]any{"x": 1}))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["error"]["code"] != string(CodePayLinkNotFound) {
		t.Fatalf("code = %v", body["error"]["code"])
	}
}

func TestWriteErrorPlainWrapped(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteError(rr, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("opaque"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestStatusForUnknown(t *testing.T) {
	if StatusFor(ErrorCode("WHATEVER")) != http.StatusInternalServerError {
		t.Fatal("unknown code should map to 500")
	}
}

func TestAppErrorStatusOverride(t *testing.T) {
	e := &AppError{Code: CodeInvalidPayload, Message: "x", HTTPStatus: http.StatusTeapot}
	if e.Status() != http.StatusTeapot {
		t.Fatalf("override not honored: %d", e.Status())
	}
	if AsAppError(nil) != nil {
		t.Fatal("AsAppError(nil) should be nil")
	}
}

func TestEnvelopeNilDetails(t *testing.T) {
	body := Envelope(context.Background(), CodeInvalidPayload, "m", nil).(envelopeBody)
	if body.Error.Details == nil {
		t.Fatal("details should default to empty object")
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		A int `json:"a"`
	}
	good := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	var p payload
	if err := DecodeJSON(httptest.NewRecorder(), good, &p); err != nil || p.A != 1 {
		t.Fatalf("good decode: %v / %+v", err, p)
	}
	bad := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))
	if err := DecodeJSON(httptest.NewRecorder(), bad, &payload{}); err == nil {
		t.Fatal("malformed JSON should error")
	}
	trailing := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}{"a":2}`))
	if err := DecodeJSON(httptest.NewRecorder(), trailing, &payload{}); err == nil {
		t.Fatal("trailing data should error")
	}
	unknown := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"b":1}`))
	if err := DecodeJSON(httptest.NewRecorder(), unknown, &payload{}); err == nil {
		t.Fatal("unknown field should error")
	}
}

func TestLoggerFromContextDefault(t *testing.T) {
	if LoggerFromContext(context.Background()) == nil {
		t.Fatal("should fall back to slog.Default")
	}
	if TraceIDFromContext(context.Background()) != "" {
		t.Fatal("missing trace id should be empty")
	}
}
