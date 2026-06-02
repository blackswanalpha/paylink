// Package server wires the chi router: middleware (correlation id, logging, recovery, metrics),
// health/readiness/metrics endpoints, and the /v1 routes (charges + the rail callback).
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	idempotency "github.com/paylink/idempotency-go"
	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/mpesa-adapter/internal/config"
	"github.com/paylink/mpesa-adapter/internal/domain"
	"github.com/paylink/mpesa-adapter/internal/httpx"
	"github.com/paylink/mpesa-adapter/internal/metrics"
)

// ReadyCheck is a named readiness dependency probe.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

// Server holds the HTTP dependencies and the built router.
type Server struct {
	svc           *domain.Service
	idem          *idempotency.Store
	metrics       *metrics.Metrics
	log           *slog.Logger
	ready         []ReadyCheck
	internalToken string // shared secret required on the rail→core callback hop ("" = no auth, dev)
	router        http.Handler
}

// New builds a Server and its router.
func New(svc *domain.Service, idem *idempotency.Store, m *metrics.Metrics, log *slog.Logger, internalToken string, ready []ReadyCheck) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{svc: svc, idem: idem, metrics: m, log: log, internalToken: internalToken, ready: ready}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// work18 — telemetry first: extract inbound trace context, start the server span, and seed
	// X-Request-Id with the trace id so the RequestID middleware below adopts it.
	r.Use(telemetry.Middleware(config.ServiceName, routeLabel))
	r.Use(httpx.RequestID(s.log))
	r.Use(httpx.RequestLogger)
	r.Use(httpx.Recoverer)
	r.Use(s.metrics.Middleware(routeLabel))

	r.Get("/internal/healthz", s.healthz)
	r.Get("/internal/readyz", s.readyz)
	r.Handle("/metrics", s.metrics.Handler())

	r.Route("/v1", func(r chi.Router) {
		r.Post("/charges", s.initiateCharge)
		r.Post("/callbacks/mpesa", s.handleCallback)
	})

	return r
}

// routeLabel returns a low-cardinality route label (chi's matched pattern) for metrics.
func routeLabel(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	failures := map[string]any{}
	for _, c := range s.ready {
		if err := c.Check(ctx); err != nil {
			failures[c.Name] = err.Error()
		}
	}
	if len(failures) > 0 {
		httpx.WriteJSON(w, http.StatusServiceUnavailable,
			httpx.Envelope(ctx, httpx.CodeServiceNotReady, "one or more dependencies are not ready", failures))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
