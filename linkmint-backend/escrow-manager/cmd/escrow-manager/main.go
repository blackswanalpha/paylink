// Command escrow-manager runs the escrow-manager service (work20): conditional PayLink
// release/refund. It exposes /v1/escrows, consumes chain.paylink.verified from the bus to mark
// escrows funded (A.3 — funding truth comes from the chain), and sweeps due time_locks/timeouts.
// It is strictly non-custodial (A.1): escrow.released / escrow.refunded are instructions for the
// settlement layer, never fund movements.
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

	"github.com/paylink/escrow-manager/internal/config"
	"github.com/paylink/escrow-manager/internal/consumer"
	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/events"
	"github.com/paylink/escrow-manager/internal/logging"
	"github.com/paylink/escrow-manager/internal/metrics"
	"github.com/paylink/escrow-manager/internal/server"
	pgstore "github.com/paylink/escrow-manager/internal/store/postgres"
	"github.com/paylink/escrow-manager/internal/sweeper"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err.Error())
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel, config.ServiceName)
	slog.SetDefault(log)

	if cfg.DevCreatorAddr != "" {
		log.Warn("ESCROW_DEV_CREATOR_ADDR is set — dev-only X-Creator-Addr fallback active")
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

	svc := domain.NewService(pg, publisher, log,
		domain.WithMetrics(m), domain.WithDefaultTimeout(cfg.DefaultTimeout))

	ready := []server.ReadyCheck{
		{Name: "postgres", Check: pg.Ping},
		{Name: "redis", Check: idem.Ping},
	}
	srv := server.New(svc, idem, m, log, ready, cfg.DevCreatorAddr)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// Bus consumer (work15/work20): chain.paylink.verified → funded flag (+ release when the
	// condition is already satisfied). Group "escrow-manager" on the "chain" topic; offsets are
	// committed only after a clean handle, and the store's DbDedupe absorbs redeliveries.
	if cfg.EventConsumerEnabled {
		if cfg.KafkaBrokers == "" {
			log.Warn("event consumer enabled but KAFKA_BROKERS is empty — consumer skipped; escrows will not auto-fund")
		} else {
			con, cerr := eventbus.NewConsumer(
				eventbus.Config{
					Brokers:  eventbus.SplitBrokers(cfg.KafkaBrokers),
					ClientID: config.ServiceName,
					GroupID:  config.ServiceName,
				},
				[]string{"chain"}, log,
			)
			if cerr != nil {
				log.Error("kafka consumer init failed", "err", cerr.Error())
				os.Exit(1)
			}
			h := consumer.New(svc, m, log)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := con.Run(ctx, h.Handle); err != nil && !errors.Is(err, context.Canceled) {
					log.Error("bus consumer stopped", "err", err.Error())
				}
			}()
			log.Info("bus consumer started", "topic", "chain", "group", config.ServiceName)
		}
	} else {
		log.Warn("event consumer disabled (ESCROW_EVENT_CONSUMER_ENABLED=false) — escrows will not auto-fund")
	}

	// Sweeper: release due funded time_locks, refund timeouts. CAS updates keep it safe to run
	// alongside confirms/funding; DISPUTED rows are never touched.
	if cfg.SweepEnabled {
		sw := sweeper.New(svc, cfg.SweepInterval, m, log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sw.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("sweeper stopped", "err", err.Error())
			}
		}()
	} else {
		log.Warn("sweeper disabled (ESCROW_SWEEP_ENABLED=false) — time_lock releases and timeout refunds will not run")
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
