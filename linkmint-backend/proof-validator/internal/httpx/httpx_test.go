package httpx_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paylink/proof-validator/internal/httpx"
)

func TestStatusFor(t *testing.T) {
	if httpx.StatusFor(httpx.CodeInvalidProofSignature) != http.StatusUnauthorized {
		t.Error("INVALID_PROOF_SIGNATURE should map to 401")
	}
	if httpx.StatusFor(httpx.CodeChainUnavailable) != http.StatusBadGateway {
		t.Error("CHAIN_UNAVAILABLE should map to 502")
	}
	if httpx.StatusFor("UNKNOWN_CODE") != http.StatusInternalServerError {
		t.Error("unknown codes should map to 500")
	}
}

func TestAppError(t *testing.T) {
	e := httpx.NewError(httpx.CodeProofExists, "dup", map[string]any{"k": "v"})
	if e.Status() != http.StatusConflict {
		t.Errorf("status = %d, want 409", e.Status())
	}
	if !strings.Contains(e.Error(), "PROOF_EXISTS") {
		t.Errorf("Error() = %q", e.Error())
	}
	override := &httpx.AppError{Code: httpx.CodeProofExists, HTTPStatus: 418}
	if override.Status() != 418 {
		t.Error("HTTPStatus override ignored")
	}
}

func TestAsAppError(t *testing.T) {
	if httpx.AsAppError(nil) != nil {
		t.Error("nil should map to nil")
	}
	ae := httpx.NewError(httpx.CodePayLinkExpired, "x", nil)
	if httpx.AsAppError(ae) != ae {
		t.Error("AppError should pass through")
	}
	if httpx.AsAppError(errors.New("boom")).Code != httpx.CodeInternalError {
		t.Error("opaque error should become INTERNAL_ERROR")
	}
}

func TestWriteError_Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	httpx.WriteError(rec, r, httpx.NewError(httpx.CodeInvalidProofShape, "bad", map[string]any{"f": "pl_id"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
			TraceID string         `json:"trace_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("envelope not JSON: %v", err)
	}
	if body.Error.Code != "INVALID_PROOF_SHAPE" || body.Error.Message != "bad" || body.Error.Details["f"] != "pl_id" {
		t.Fatalf("unexpected envelope: %+v", body.Error)
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		A int `json:"a"`
	}
	t.Run("valid", func(t *testing.T) {
		var p payload
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
		if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &p); err != nil || p.A != 1 {
			t.Fatalf("decode: %v, p=%+v", err, p)
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		var p payload
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1,"b":2}`))
		if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &p); httpx.AsAppError(err).Code != httpx.CodeInvalidPayload {
			t.Fatalf("want INVALID_PAYLOAD, got %v", err)
		}
	})
	t.Run("trailing data", func(t *testing.T) {
		var p payload
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}{}`))
		if err := httpx.DecodeJSON(httptest.NewRecorder(), r, &p); err == nil {
			t.Fatal("expected error on trailing data")
		}
	})
}

func TestMiddleware(t *testing.T) {
	t.Run("request id echoed", func(t *testing.T) {
		h := httpx.RequestID(slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if httpx.TraceIDFromContext(r.Context()) == "" {
				t.Error("trace id not in context")
			}
			w.WriteHeader(http.StatusOK)
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Header().Get("X-Request-Id") == "" {
			t.Error("X-Request-Id not set")
		}
	})

	t.Run("propagates incoming id", func(t *testing.T) {
		h := httpx.RequestID(slog.Default())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-Id", "abc123")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Header().Get("X-Request-Id") != "abc123" {
			t.Error("incoming request id not propagated")
		}
	})

	t.Run("recoverer", func(t *testing.T) {
		h := httpx.RequestID(slog.Default())(httpx.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("panic should yield 500, got %d", rec.Code)
		}
	})

	t.Run("logger", func(t *testing.T) {
		h := httpx.RequestID(slog.Default())(httpx.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
	})
}
