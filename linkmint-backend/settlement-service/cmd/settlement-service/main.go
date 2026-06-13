// Command settlement-service runs the settlement-service (work23): off-chain settlement lifecycle.
// It aggregates verified PayLinks (chain.paylink.verified) into per-merchant settlements with
// gross/fee/net, attaches the chain fee (chain.fee.collected), schedules T+1 payouts, ingests rail
// settlement files, and records every flow as a balanced double-entry ledger posting (work16, A.6).
// It is strictly non-custodial (A.1): a payout is an INSTRUCTION over the merchant's external rail,
// never a fund movement. It also consumes merchant.* (routing projections) and
// refund.clawback.requested (negative offsets).
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

	"github.com/paylink/settlement-service/internal/config"
	"github.com/paylink/settlement-service/internal/consumer"
	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/events"
	"github.com/paylink/settlement-service/internal/logging"
	"github.com/paylink/settlement-service/internal/metrics"
	"github.com/paylink/settlement-service/internal/scheduler"
	"github.com/paylink/settlement-service/internal/server"
	pgstore "github.com/paylink/settlement-service/internal/store/postgres"
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
		log.Warn("SETTLEMENT_DEV_CREATOR_ADDR is set — dev-only X-Creator-Addr fallback active")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set; never fatal.
	otelShutdown, err := telemetry.Init(ctx, config.ServiceName, "0.1.0")
	if err != nil {
		log.Warn("telemetry init failed; tracing disabled", "err", err.Error())
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	// PostgreSQL store + migrations (settlement schema + shared ledger schema).
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

	// Redis-backed idempotency (HTTP Idempotency-Key store).
	rc, err := idempotency.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer rc.Close()
	idem := idempotency.New(rc, config.ServiceName, cfg.IdempotencyTTL)

	// Domain-event publisher (work15). "log" is the in-process seam; "kafka" produces to the bus.
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
		domain.WithMetrics(m),
		domain.WithCurrency(cfg.Currency),
		domain.WithTimezone(cfg.DefaultTZ),
		domain.WithPlatformFeeBps(cfg.PlatformFeeBps),
		domain.WithDefaultRail(cfg.DefaultRail),
		domain.WithMinPayout(cfg.MinPayoutFor),
	)

	ready := []server.ReadyCheck{
		{Name: "postgres", Check: pg.Ping},
		{Name: "redis", Check: idem.Ping},
	}
	srv := server.New(svc, idem, m, log, ready, cfg.DevCreatorAddr, cfg.IngestToken)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// Bus consumers (work15): chain (verified + fee), merchant (projections), refund (clawback).
	// Offsets commit only after a clean handle; the store's DbDedupe absorbs redeliveries.
	if cfg.EventConsumerEnabled {
		if cfg.KafkaBrokers == "" {
			log.Warn("event consumer enabled but KAFKA_BROKERS is empty — consumer skipped; settlements will not aggregate")
		} else {
			con, cerr := eventbus.NewConsumer(
				eventbus.Config{
					Brokers:  eventbus.SplitBrokers(cfg.KafkaBrokers),
					ClientID: config.ServiceName,
					GroupID:  config.ServiceName,
				},
				consumer.Topics, log,
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
			log.Info("bus consumer started", "topics", consumer.Topics, "group", config.ServiceName)
		}
	} else {
		log.Warn("event consumer disabled (SETTLEMENT_EVENT_CONSUMER_ENABLED=false) — settlements will not aggregate")
	}

	// Payout scheduler: close due settlements (T+1 cutoff) and instruct payouts. CAS-safe alongside
	// the consumers.
	if cfg.ScheduleEnabled {
		sch := scheduler.New(svc, cfg.ScheduleInterval, m, log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sch.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("scheduler stopped", "err", err.Error())
			}
		}()
	} else {
		log.Warn("scheduler disabled (SETTLEMENT_SCHEDULE_ENABLED=false) — payouts will not be instructed")
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

	select {
	case <-srvErr:
		os.Exit(1)
	default:
	}
}
