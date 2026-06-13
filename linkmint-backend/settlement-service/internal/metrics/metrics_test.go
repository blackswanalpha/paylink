package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsRecordAndScrape(t *testing.T) {
	m := New()
	m.EventConsumed("settled")
	m.Payout("instructed")
	m.ScheduleTick()

	// Exercise the HTTP middleware so http_requests_total is populated.
	h := m.Middleware(func(*http.Request) string { return "/v1/settlements" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/settlements", nil))

	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()
	for _, want := range []string{
		"settlement_events_consumed_total", "settlement_payouts_total",
		"settlement_schedule_ticks_total", "http_requests_total",
	} {
		if !contains(body, want) {
			t.Errorf("scrape missing %q", want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
