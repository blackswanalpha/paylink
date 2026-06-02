// Package server wires the mirror's minimal internal HTTP surface: health, readiness, and metrics.
// It is INTERNAL-ONLY (no /v1, never routed through the api-gateway) — the mirror is a background
// worker, like payment-orchestrator's /internal surface and the audit-log intake.
package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/paylink/chain-event-mirror/internal/metrics"
)

// ReadyCheck is a named readiness dependency probe.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

// Server holds the HTTP dependencies and the built router.
type Server struct {
	ready  []ReadyCheck
	router http.Handler
}

// New builds a Server and its router.
func New(m *metrics.Metrics, ready []ReadyCheck) *Server {
	s := &Server{ready: ready}
	r := chi.NewRouter()
	r.Get("/internal/healthz", s.healthz)
	r.Get("/internal/readyz", s.readyz)
	r.Handle("/metrics", m.Handler())
	s.router = r
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string, len(s.ready))
	code := http.StatusOK
	for _, c := range s.ready {
		if err := c.Check(r.Context()); err != nil {
			checks[c.Name] = err.Error()
			code = http.StatusServiceUnavailable
		} else {
			checks[c.Name] = "ok"
		}
	}
	status := "ready"
	if code != http.StatusOK {
		status = "not_ready"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": checks})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
