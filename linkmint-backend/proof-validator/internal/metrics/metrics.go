// Package metrics exposes Prometheus collectors and an HTTP middleware. Each Metrics owns
// its own registry so tests can construct instances without global-registry collisions.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics bundles the service's Prometheus collectors.
type Metrics struct {
	reg            *prometheus.Registry
	httpRequests   *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	proofsReceived *prometheus.CounterVec
	settlementTx   *prometheus.CounterVec
	chainTxs       *prometheus.CounterVec
}

// New builds a Metrics with a fresh registry and registered collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route, and status.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),
		proofsReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proofs_received_total",
			Help: "Proofs received on /v1/proofs by outcome.",
		}, []string{"result"}),
		settlementTx: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "settlement_tx_total",
			Help: "Settlement transactions broadcast to the lVM by result.",
		}, []string{"result"}),
		// work18 — the standard cross-service counter for chain submissions; mirrors settlement_tx_total
		// under the spec's shared name so one dashboard query spans every chain-submitting service.
		chainTxs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "chain_txs_submitted_total",
			Help: "Transactions submitted to the lVM chain by result.",
		}, []string{"result"}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.proofsReceived, m.settlementTx, m.chainTxs)
	return m
}

// Handler returns the Prometheus scrape handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// ProofReceived records the outcome of a submitted proof
// ("accepted", "rejected_shape", "rejected_signature", "already_settled", "rejected_paylink",
// "chain_unavailable", "error").
func (m *Metrics) ProofReceived(result string) {
	m.proofsReceived.WithLabelValues(result).Inc()
}

// SettlementTx records a settlement broadcast attempt ("broadcast" | "error"). It also feeds the
// standard chain_txs_submitted_total (work18) so the result is visible under the shared metric name.
func (m *Metrics) SettlementTx(result string) {
	m.settlementTx.WithLabelValues(result).Inc()
	m.chainTxs.WithLabelValues(result).Inc()
}

// Middleware records HTTP request counts and durations. routeOf maps a request to a low
// cardinality route label (e.g. chi's matched pattern).
func (m *Metrics) Middleware(routeOf func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			route := routeOf(r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			m.httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
			m.httpDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}
