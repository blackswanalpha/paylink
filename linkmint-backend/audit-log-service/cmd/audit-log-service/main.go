// Command audit-log-service runs the audit-log-service (work13): an append-only, tamper-evident
// hash chain that is the system of record for "who did what when" across LinkMint. It exposes
// /v1/audit-log (internal intake + admin/compliance reads + chain verification), consumes
// audit.intake (a work15 seam), and is non-custodial (A.1) and strictly append-only.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/paylink/audit-log-service/internal/auth"
	"github.com/paylink/audit-log-service/internal/config"
	"github.com/paylink/audit-log-service/internal/domain"
	"github.com/paylink/audit-log-service/internal/events"
	"github.com/paylink/audit-log-service/internal/idempotency"
	"github.com/paylink/audit-log-service/internal/intake"
	"github.com/paylink/audit-log-service/internal/logging"
	"github.com/paylink/audit-log-service/internal/metrics"
	"github.com/paylink/audit-log-service/internal/server"
	pgstore "github.com/paylink/audit-log-service/internal/store/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err.Error())
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel, config.ServiceName)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// PostgreSQL store + migrations.
	pg, err := pgstore.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer pg.Close()
	if err := pg.Migrate(ctx); err != nil {
		log.Error("migrations failed", "err", err.Error())
		os.Exit(1)
	}
	if head, count, terr := pg.Tail(ctx); terr == nil {
		log.Info("audit chain head", "entries", count, "head_hash", hex.EncodeToString(head))
	}

	// Redis-backed idempotency (Idempotency-Key is optional on intake but the store backs readiness).
	rc, err := idempotency.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer rc.Close()
	idem := idempotency.New(rc, cfg.IdempotencyTTL)

	// RS256 reader verification (config-gated — disabled => gateway-trust).
	verifier, err := auth.New(cfg.JWTPublicKeyPEM, cfg.JWTIssuer, cfg.JWTAudience, cfg.ReaderRoles)
	if err != nil {
		log.Error("jwt verifier init failed", "err", err.Error())
		os.Exit(1)
	}
	if verifier.Enabled() {
		log.Info("read endpoints verify RS256 in-service", "reader_roles", cfg.ReaderRoles)
	} else {
		log.Warn("read endpoints trust the gateway (AUDIT_JWT_PUBLIC_KEY_PEM unset) — no in-service JWT verification")
	}
	if cfg.InternalSharedSecret != "" {
		log.Info("intake gate enabled (X-Internal-Token required on POST /v1/audit-log)")
	} else {
		log.Warn("intake gate disabled (AUDIT_INTERNAL_SHARED_SECRET unset) — trusted network only")
	}

	pub := events.NewLogPublisher(log)
	m := metrics.New()
	svc := domain.NewService(pg, pub, log, domain.WithMetrics(m))

	ready := []server.ReadyCheck{
		{Name: "postgres", Check: pg.Ping},
		{Name: "redis", Check: idem.Ping},
	}
	srv := server.New(svc, idem, m, verifier, cfg.InternalSharedSecret, log, ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// audit.intake consumer (work15 NATS seam). The HTTP POST is the live Phase-1 intake; this is a
	// no-op stub until the event bus lands.
	if cfg.IntakeEnabled {
		src := intake.NoopSource{Log: log}
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := func(ctx context.Context, in domain.AppendInput) error {
				_, err := svc.Append(ctx, in)
				return err
			}
			if err := src.Run(ctx, handler); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("intake consumer stopped", "err", err.Error())
			}
		}()
	}

	srvErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err.Error())
			srvErr <- err
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", "err", err.Error())
	}
	wg.Wait()
	log.Info("stopped")

	// Exit non-zero if the server died on a listen/serve error (so probes/orchestrators notice).
	select {
	case <-srvErr:
		os.Exit(1)
	default:
	}
}
