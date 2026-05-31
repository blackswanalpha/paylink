package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paylink/proof-validator/internal/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestMetrics_RegistersAndCounts(t *testing.T) {
	m := metrics.New()
	m.ProofReceived("accepted")
	m.SettlementTx("broadcast")
	body := scrape(t, m)
	// http_requests_total has no series until the middleware runs (see TestMetrics_Middleware);
	// a CounterVec is only emitted after its first observation.
	for _, want := range []string{"proofs_received_total", "settlement_tx_total", `result="accepted"`, `result="broadcast"`} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics scrape missing %q\n%s", want, body)
		}
	}
}

func TestMetrics_Middleware(t *testing.T) {
	m := metrics.New()
	h := m.Middleware(func(*http.Request) string { return "/v1/proofs" })(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/proofs", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if body := scrape(t, m); !strings.Contains(body, `route="/v1/proofs"`) {
		t.Fatalf("expected the request to be counted by route\n%s", body)
	}
}
