// Package metrics exposes the mirror's Prometheus collectors. Each Metrics owns its own registry so
// tests can construct instances without global-registry collisions.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics bundles the mirror's collectors.
type Metrics struct {
	reg      *prometheus.Registry
	mirrored *prometheus.CounterVec
}

// New builds a Metrics with a fresh registry and registered collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		mirrored: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "chain_events_mirrored_total",
			Help: "Chain events mirrored to the bus by kind and result (ok|error).",
		}, []string{"kind", "result"}),
	}
	reg.MustRegister(m.mirrored)
	return m
}

// Handler returns the Prometheus scrape handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// Mirrored records a mirrored chain event by kind and result.
func (m *Metrics) Mirrored(kind, result string) {
	m.mirrored.WithLabelValues(kind, result).Inc()
}
