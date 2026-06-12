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
	transitions    *prometheus.CounterVec
	eventsConsumed *prometheus.CounterVec
	sweeps         prometheus.Counter
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
		transitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "escrow_transitions_total",
			Help: "Escrow FSM transitions by kind (conditions_met, release, timeout, dispute).",
		}, []string{"kind"}),
		eventsConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "escrow_events_consumed_total",
			Help: "Bus events consumed by result (funded, released, duplicate, skipped, ignored, error).",
		}, []string{"result"}),
		sweeps: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "escrow_sweeps_total",
			Help: "Sweeper ticks executed (release-due time_locks + timeout refunds).",
		}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.transitions, m.eventsConsumed, m.sweeps)
	return m
}

// Handler returns the Prometheus scrape handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// Transition records an escrow FSM transition by kind.
func (m *Metrics) Transition(kind string) {
	m.transitions.WithLabelValues(kind).Inc()
}

// EventConsumed records a consumed bus event by result.
func (m *Metrics) EventConsumed(result string) {
	m.eventsConsumed.WithLabelValues(result).Inc()
}

// SweepTick records one sweeper tick.
func (m *Metrics) SweepTick() {
	m.sweeps.Inc()
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
