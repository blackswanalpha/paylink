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
	eventsConsumed *prometheus.CounterVec
	payouts        *prometheus.CounterVec
	schedules      prometheus.Counter
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
		eventsConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "settlement_events_consumed_total",
			Help: "Bus events consumed by result (settled, fee, merchant, bank, clawback, duplicate, ignored, error).",
		}, []string{"result"}),
		payouts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "settlement_payouts_total",
			Help: "Payouts by status transition (scheduled, instructed, paid, failed).",
		}, []string{"status"}),
		schedules: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "settlement_schedule_ticks_total",
			Help: "Scheduler ticks executed (close due settlements + instruct payouts).",
		}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.eventsConsumed, m.payouts, m.schedules)
	return m
}

// Handler returns the Prometheus scrape handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// EventConsumed records a consumed bus event by result.
func (m *Metrics) EventConsumed(result string) {
	m.eventsConsumed.WithLabelValues(result).Inc()
}

// Payout records a payout status transition.
func (m *Metrics) Payout(status string) {
	m.payouts.WithLabelValues(status).Inc()
}

// ScheduleTick records one scheduler tick.
func (m *Metrics) ScheduleTick() {
	m.schedules.Inc()
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
