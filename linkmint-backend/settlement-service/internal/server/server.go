// Package server wires the chi router: middleware (telemetry, correlation id, logging, recovery,
// metrics), health/readiness/metrics endpoints, the merchant-scoped /v1 routes, and the internal
// rail-file ingest endpoint.
package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	idempotency "github.com/paylink/idempotency-go"
	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/settlement-service/internal/config"
	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/httpx"
	"github.com/paylink/settlement-service/internal/metrics"
)

// ReadyCheck is a named readiness dependency probe.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

// Server holds the HTTP dependencies and the built router.
type Server struct {
	svc     *domain.Service
	idem    *idempotency.Store
	metrics *metrics.Metrics
	log     *slog.Logger
	ready   []ReadyCheck
	// devCreatorAddr is the SETTLEMENT_DEV_CREATOR_ADDR fallback for the gateway-injected
	// X-Creator-Addr header. Empty (the deployed default) ⇒ merchant routes without it get 401.
	devCreatorAddr string
	// ingestToken guards the internal rail-file ingest endpoint. Empty ⇒ open (local dev only).
	ingestToken string
	router      http.Handler
}

// New builds a Server and its router.
func New(svc *domain.Service, idem *idempotency.Store, m *metrics.Metrics, log *slog.Logger, ready []ReadyCheck, devCreatorAddr, ingestToken string) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		svc: svc, idem: idem, metrics: m, log: log, ready: ready,
		devCreatorAddr: devCreatorAddr, ingestToken: ingestToken,
	}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(telemetry.Middleware(config.ServiceName, routeLabel))
	r.Use(httpx.RequestID(s.log))
	r.Use(httpx.RequestLogger)
	r.Use(httpx.Recoverer)
	r.Use(s.metrics.Middleware(routeLabel))

	r.Get("/internal/healthz", s.healthz)
	r.Get("/internal/readyz", s.readyz)
	r.Handle("/metrics", s.metrics.Handler())

	// Internal/trusted-network rail-file ingest (NOT exposed via the gateway). Guarded by a shared
	// token; mTLS terminates upstream (ADR-009 trusted-network pattern).
	r.Post("/settlements/files/ingest", s.ingestRailFile)

	// Merchant-scoped read/write API (X-Creator-Addr injected by the gateway after jwt/key-auth).
	r.Route("/v1", func(r chi.Router) {
		r.Get("/settlements", s.listSettlements)
		r.Get("/settlements/{id}", s.getSettlement)
		r.Get("/payouts", s.listPayouts)
		r.Get("/payouts/{id}", s.getPayout)
		r.Post("/payouts", s.createPayout)
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

const creatorAddrHeader = "X-Creator-Addr"

// callerAddr resolves the caller's on-chain address (the merchant settlement key): the
// gateway-injected X-Creator-Addr (the gateway strips any client value and injects the authenticated
// one), falling back to SETTLEMENT_DEV_CREATOR_ADDR for local dev. "" ⇒ unauthenticated.
func (s *Server) callerAddr(r *http.Request) string {
	if v := r.Header.Get(creatorAddrHeader); v != "" {
		return v
	}
	return s.devCreatorAddr
}

// requireCaller writes a 401 envelope and returns "" when no caller identity is present.
func (s *Server) requireCaller(w http.ResponseWriter, r *http.Request) string {
	addr := s.callerAddr(r)
	if addr == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized,
			"missing caller identity (X-Creator-Addr)", nil))
	}
	return addr
}
