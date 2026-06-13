package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusFor(t *testing.T) {
	if StatusFor(CodeSettlementNotFound) != http.StatusNotFound {
		t.Error("not-found should map to 404")
	}
	if StatusFor(CodeIdempotentConflict) != http.StatusConflict {
		t.Error("idempotent conflict should map to 409")
	}
	if StatusFor(ErrorCode("UNKNOWN")) != http.StatusInternalServerError {
		t.Error("unknown code should map to 500")
	}
}

func TestAsAppError(t *testing.T) {
	ae := NewError(CodeInvalidPayload, "bad", nil)
	if AsAppError(ae) != ae {
		t.Error("AppError should pass through")
	}
	if AsAppError(nil) != nil {
		t.Error("nil should stay nil")
	}
	wrapped := AsAppError(context.Canceled)
	if wrapped.Code != CodeInternalError {
		t.Errorf("opaque error code=%s, want INTERNAL_ERROR", wrapped.Code)
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	WriteError(rr, r, NewError(CodePayoutNotFound, "nope", map[string]any{"id": "x"}))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rr.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"]["code"] != "PAYOUT_NOT_FOUND" {
		t.Fatalf("envelope code=%v", body["error"]["code"])
	}
}

func TestDecodeJSONStrict(t *testing.T) {
	type payload struct {
		A string `json:"a"`
	}
	var p payload
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"ok"}`))
	if err := DecodeJSON(rr, r, &p); err != nil {
		t.Fatalf("valid body: %v", err)
	}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"x","b":1}`))
	if err := DecodeJSON(rr, r, &p); err == nil {
		t.Fatal("unknown field should error")
	}
}
