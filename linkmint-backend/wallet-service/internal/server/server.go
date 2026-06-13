// Package server wires the chi router: middleware (telemetry, correlation id, logging, recovery,
// metrics), health/readiness/metrics endpoints, the self-scoped /v1 wallet & staking routes, and the
// public /v1/treasury/stats route.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/wallet-service/internal/config"
	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/httpx"
	"github.com/paylink/wallet-service/internal/metrics"
)

// ReadyCheck is a named readiness dependency probe.
type ReadyCheck struct {
	Name  string
	Check func(context.Context) error
}

// Server holds the HTTP dependencies and the built router.
type Server struct {
	svc     *domain.Service
	metrics *metrics.Metrics
	log     *slog.Logger
	// ready are HARD readiness deps (postgres): a failure makes readyz 503.
	ready []ReadyCheck
	// chainPing is a SOFT dep: when it fails, readyz stays 200 but reports the chain as degraded
	// (the indexed read-side still serves history/staking/treasury and cached balances).
	chainPing func(context.Context) error
	// devCreatorAddr is the WALLET_DEV_CREATOR_ADDR fallback for the gateway-injected X-Creator-Addr.
	devCreatorAddr string
	router         http.Handler
}

// New builds a Server and its router.
func New(svc *domain.Service, m *metrics.Metrics, log *slog.Logger, ready []ReadyCheck, chainPing func(context.Context) error, devCreatorAddr string) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		svc: svc, metrics: m, log: log, ready: ready,
		chainPing: chainPing, devCreatorAddr: devCreatorAddr,
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

	r.Route("/v1", func(r chi.Router) {
		// Self-scoped wallet + staking reads (gateway injects the authenticated X-Creator-Addr).
		r.Get("/wallets/{addr}", s.getWallet)
		r.Get("/wallets/{addr}/transactions", s.listTransactions)
		r.Route("/staking", func(r chi.Router) {
			r.Get("/positions", s.getPositions)
			r.Get("/rewards", s.getRewards)
			r.Post("/intent", s.postIntent)
		})
		// Public treasury stats (no auth).
		r.Get("/treasury/stats", s.getTreasuryStats)
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
	// The chain is a soft dependency: the read-side serves while it is down. Report it as degraded
	// without failing readiness.
	body := map[string]any{"status": "ready"}
	if s.chainPing != nil {
		if err := s.chainPing(ctx); err != nil {
			body["degraded"] = map[string]any{"chain": err.Error()}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, body)
}

const creatorAddrHeader = "X-Creator-Addr"

// callerAddr resolves the caller's on-chain address: the gateway-injected X-Creator-Addr (the gateway
// strips any client value and injects the authenticated one), falling back to WALLET_DEV_CREATOR_ADDR
// for local dev. "" ⇒ unauthenticated.
func (s *Server) callerAddr(r *http.Request) string {
	if v := r.Header.Get(creatorAddrHeader); v != "" {
		return v
	}
	return s.devCreatorAddr
}

// requireSelf authorizes a self-scoped request: the caller must be authenticated and own `target`.
// It writes the appropriate envelope and returns false when the request is not authorized.
func (s *Server) requireSelf(w http.ResponseWriter, r *http.Request, target string) bool {
	caller := s.callerAddr(r)
	if caller == "" {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeUnauthorized, "missing caller identity (X-Creator-Addr)", nil))
		return false
	}
	norm, err := domain.NormalizeAddr(target)
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidAddress, "invalid address", nil))
		return false
	}
	if !equalAddr(caller, norm) {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeForbidden, "address does not match the authenticated caller", nil))
		return false
	}
	return true
}

func equalAddr(a, b string) bool {
	an, err1 := domain.NormalizeAddr(a)
	bn, err2 := domain.NormalizeAddr(b)
	return err1 == nil && err2 == nil && an == bn
}

// mapErr maps a domain sentinel error to the HTTP envelope (unknown → opaque 500 via httpx).
func mapErr(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidAddress):
		return httpx.NewError(httpx.CodeInvalidAddress, "invalid address", nil)
	case errors.Is(err, domain.ErrInvalidAmount):
		return httpx.NewError(httpx.CodeInvalidAmount, err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidAction):
		return httpx.NewError(httpx.CodeInvalidPayload, "action must be \"stake\" or \"unstake\"", nil)
	case errors.Is(err, domain.ErrChainUnavailable):
		return httpx.NewError(httpx.CodeChainUnavailable, "chain rpc is unavailable", nil)
	case errors.Is(err, domain.ErrNotFound):
		return httpx.NewError(httpx.CodeWalletNotFound, "not found", nil)
	default:
		return err
	}
}
