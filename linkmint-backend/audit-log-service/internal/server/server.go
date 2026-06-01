// Package server wires the chi router: middleware (correlation id, logging, recovery, metrics),
// health/readiness/metrics endpoints, the internal intake gate, and the /v1/audit-log routes.
package server

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/paylink/audit-log-service/internal/auth"
	"github.com/paylink/audit-log-service/internal/domain"
	"github.com/paylink/audit-log-service/internal/httpx"
	"github.com/paylink/audit-log-service/internal/idempotency"
	"github.com/paylink/audit-log-service/internal/metrics"
)

// ReadyCheck is a named readiness dependency probe.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

// Server holds the HTTP dependencies and the built router.
type Server struct {
	svc            *domain.Service
	idem           *idempotency.Store
	metrics        *metrics.Metrics
	verifier       *auth.Verifier
	internalSecret string
	log            *slog.Logger
	ready          []ReadyCheck
	router         http.Handler
}

// New builds a Server and its router.
func New(svc *domain.Service, idem *idempotency.Store, m *metrics.Metrics, verifier *auth.Verifier, internalSecret string, log *slog.Logger, ready []ReadyCheck) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		svc:            svc,
		idem:           idem,
		metrics:        m,
		verifier:       verifier,
		internalSecret: internalSecret,
		log:            log,
		ready:          ready,
	}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(httpx.RequestID(s.log))
	r.Use(httpx.RequestLogger)
	r.Use(httpx.Recoverer)
	r.Use(s.metrics.Middleware(routeLabel))

	r.Get("/internal/healthz", s.healthz)
	r.Get("/internal/readyz", s.readyz)
	r.Handle("/metrics", s.metrics.Handler())

	r.Route("/v1", func(r chi.Router) {
		// Intake (internal). mTLS in the spec → ADR-009 X-Internal-Token stand-in.
		r.With(s.internalGate).Post("/audit-log", s.postEntry)

		// Reads (admin/compliance). RS256 verified in-service when configured, else gateway-trust.
		r.Group(func(r chi.Router) {
			r.Use(s.verifier.RequireReader)
			r.Get("/audit-log", s.listEntries)
			r.Get("/audit-log/verify", s.verifyChain) // static route before the {entry_id} param
			r.Get("/audit-log/{entry_id}", s.getEntry)
		})
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

// internalGate guards the intake POST. When AUDIT_INTERNAL_SHARED_SECRET is set, a constant-time
// X-Internal-Token match is required; when unset, the trusted network is the control (ADR-009).
func (s *Server) internalGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.internalSecret == "" {
			next.ServeHTTP(w, r)
			return
		}
		presented := r.Header.Get("X-Internal-Token")
		if subtle.ConstantTimeCompare([]byte(presented), []byte(s.internalSecret)) != 1 {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized, "invalid or missing X-Internal-Token", nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}
