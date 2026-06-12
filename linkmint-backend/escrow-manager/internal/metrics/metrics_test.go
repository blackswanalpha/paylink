package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecordersAndScrape(t *testing.T) {
	m := New()
	m.Transition("release")
	m.EventConsumed("funded")
	m.SweepTick()

	// Drive the HTTP middleware once.
	h := m.Middleware(func(*http.Request) string { return "/v1/escrows" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/escrows", nil))

	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()
	for _, want := range []string{
		"escrow_transitions_total", "escrow_events_consumed_total", "escrow_sweeps_total", "http_requests_total",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape missing %q", want)
		}
	}
}

func TestMiddlewareDefaultStatus(t *testing.T) {
	m := New()
	h := m.Middleware(func(*http.Request) string { return "/x" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("implicit 200 expected, got %d", rr.Code)
	}
}
