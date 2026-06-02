// Command payment-orchestrator runs the payment-orchestrator service (work02): the conductor of
// the PayLink payment lifecycle. It exposes /v1/payments, consumes lVM chain events to advance
// lifecycle state, and reconciles against on-chain truth. It is non-custodial (A.1) and treats
// chain quorum as the sole settlement authority (A.3).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	eventbus "github.com/paylink/eventbus-go"
	idempotency "github.com/paylink/idempotency-go"
	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/payment-orchestrator/internal/chain"
	"github.com/paylink/payment-orchestrator/internal/chain/wsstream"
	"github.com/paylink/payment-orchestrator/internal/config"
	"github.com/paylink/payment-orchestrator/internal/domain"
	"github.com/paylink/payment-orchestrator/internal/events"
	"github.com/paylink/payment-orchestrator/internal/logging"
	"github.com/paylink/payment-orchestrator/internal/metrics"
	"github.com/paylink/payment-orchestrator/internal/paylinks"
	"github.com/paylink/payment-orchestrator/internal/server"
	pgstore "github.com/paylink/payment-orchestrator/internal/store/postgres"
	"github.com/paylink/payment-orchestrator/internal/subscriber"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err.Error())
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel, config.ServiceName)
	slog.SetDefault(log)

	// Rail-adapter registry (work04). Config-only: the orchestrator records where a rail's adapter
	// lives but does not call it (rail stays an opaque routing label; A.4). The adapter is the entry
	// point for rail charges/callbacks. Logged so the wiring is observable at boot.
	if cfg.AdapterMpesaURL != "" {
		log.Info("rail adapter registered", "rail", "mpesa", "url", cfg.AdapterMpesaURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set; never fatal.
	otelShutdown, err := telemetry.Init(ctx, config.ServiceName, "0.1.0")
	if err != nil {
		log.Warn("telemetry init failed; tracing disabled", "err", err.Error())
	}
	defer func() { _ = otelShutdown(context.Background()) }()

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

	// Redis-backed idempotency.
	rc, err := idempotency.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer rc.Close()
	idem := idempotency.New(rc, config.ServiceName, cfg.IdempotencyTTL)

	// Outbound clients. The transport injects W3C trace context so a call to paylink-service / the
	// chain continues this request's trace (work18).
	hc := &http.Client{Timeout: cfg.HTTPTimeout, Transport: telemetry.WrapTransport(http.DefaultTransport)}
	chainClient := chain.NewClient(cfg.ChainRPCURL, hc)
	plClient := paylinks.NewClient(cfg.PayLinkServiceURL, hc)

	// Domain-event publisher (work15). Default "log" is the in-process seam; "kafka" produces to the
	// bus via eventbus-go (its Publish matches domain.Publisher exactly, so it drops in unchanged).
	var publisher domain.Publisher = events.NewLogPublisher(log)
	if cfg.EventPublisherMode == "kafka" {
		kpub, perr := eventbus.NewPublisher(
			eventbus.Config{Brokers: eventbus.SplitBrokers(cfg.KafkaBrokers), ClientID: config.ServiceName},
			config.ServiceName, log,
		)
		if perr != nil {
			log.Error("kafka publisher init failed", "err", perr.Error())
			os.Exit(1)
		}
		defer kpub.Close()
		publisher = kpub
		log.Info("event publisher: kafka", "brokers", cfg.KafkaBrokers)
	}
	m := metrics.New()

	svc := domain.NewService(pg, plClient, chainClient, publisher, log, domain.WithMetrics(m))

	ready := []server.ReadyCheck{
		{Name: "postgres", Check: pg.Ping},
		{Name: "redis", Check: idem.Ping},
		{Name: "chain", Check: chainClient.Ping},
	}
	srv := server.New(svc, idem, m, log, ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// Chain event subscriber (advances lifecycle on settle/cancel/fail).
	if cfg.ChainEventsEnabled {
		src := wsstream.New(cfg.ChainWSURL, log)
		sub := subscriber.New(src, svc, m, log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sub.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("chain subscriber stopped", "err", err.Error())
			}
		}()
	} else {
		log.Warn("chain events disabled (PAYMENT_CHAIN_EVENTS_ENABLED=false) — lifecycle advances via read reconcile only")
	}

	// HTTP server. A bind/serve failure triggers shutdown and a non-zero exit (below).
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
