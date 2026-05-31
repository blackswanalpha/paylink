package paylinks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paylink/payment-orchestrator/internal/httpx"
)

func TestGetPayLinkFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/paylinks/") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"pl_id":"0xabc","status":"CREATED","expiry":"2026-06-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	rec, err := c.GetPayLink(t.Context(), "0xabc")
	if err != nil || rec == nil {
		t.Fatalf("got rec=%v err=%v", rec, err)
	}
	if rec.ID != "0xabc" || rec.Status != "CREATED" {
		t.Fatalf("unexpected record %+v", rec)
	}
	if rec.Expiry.IsZero() || rec.Expiry.Year() != 2026 {
		t.Fatalf("expiry not parsed: %v", rec.Expiry)
	}
}

func TestGetPayLinkNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"PAYLINK_NOT_FOUND"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	rec, err := c.GetPayLink(t.Context(), "0xabc")
	if err != nil || rec != nil {
		t.Fatalf("404 should be (nil,nil); got rec=%v err=%v", rec, err)
	}
}

func TestGetPayLinkServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.GetPayLink(t.Context(), "0xabc")
	if httpx.AsAppError(err).Code != httpx.CodePayLinkSvcUnavail {
		t.Fatalf("want PAYLINK_SERVICE_UNAVAILABLE, got %v", err)
	}
}

func TestGetPayLinkTransportError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", http.DefaultClient)
	_, err := c.GetPayLink(t.Context(), "0xabc")
	if httpx.AsAppError(err).Code != httpx.CodePayLinkSvcUnavail {
		t.Fatalf("want PAYLINK_SERVICE_UNAVAILABLE, got %v", err)
	}
}
