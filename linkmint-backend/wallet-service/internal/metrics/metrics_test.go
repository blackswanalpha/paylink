package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerAndCounters(t *testing.T) {
	m := New()
	m.EventConsumed("processed")
	m.EventConsumed("duplicate")
	m.IntentBuilt("stake")
	m.IntentBuilt("unstake")

	// Exercise the HTTP middleware so http_requests_total/duration are populated.
	wrapped := m.Middleware(func(*http.Request) string { return "/v1/treasury/stats" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/treasury/stats", nil))

	scrape := httptest.NewRecorder()
	m.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := scrape.Body.String()
	for _, want := range []string{
		"wallet_events_consumed_total",
		"wallet_staking_intents_total",
		"http_requests_total",
		"http_request_duration_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape missing %q", want)
		}
	}
}

func TestMiddlewareDefaultsStatusWhenUnset(t *testing.T) {
	m := New()
	wrapped := m.Middleware(func(*http.Request) string { return "/x" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("no explicit WriteHeader"))
		}))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}
