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
	reg          *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	charges      *prometheus.CounterVec
	callbacks    *prometheus.CounterVec
	broadcasts   *prometheus.CounterVec
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
		charges: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mpesa_charges_initiated_total",
			Help: "STK-push charges initiated on /v1/charges by outcome.",
		}, []string{"result"}),
		callbacks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mpesa_callbacks_received_total",
			Help: "Daraja callbacks received on /v1/callbacks/mpesa by outcome.",
		}, []string{"result"}),
		broadcasts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mpesa_proof_broadcasts_total",
			Help: "Signed proofs broadcast to the proof-validator by result.",
		}, []string{"result"}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.charges, m.callbacks, m.broadcasts)
	return m
}

// Handler returns the Prometheus scrape handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// ChargeInitiated records a /v1/charges outcome ("accepted" | "rejected" | "daraja_error").
func (m *Metrics) ChargeInitiated(result string) { m.charges.WithLabelValues(result).Inc() }

// CallbackReceived records a callback outcome ("broadcast" | "duplicate" | "no_correlation" |
// "failed_payment" | "rejected" | "error").
func (m *Metrics) CallbackReceived(result string) { m.callbacks.WithLabelValues(result).Inc() }

// ProofBroadcast records a broadcast result ("broadcast" | "already_settled" | "rejected" | "error").
func (m *Metrics) ProofBroadcast(result string) { m.broadcasts.WithLabelValues(result).Inc() }

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
