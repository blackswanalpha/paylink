// Command chain-event-mirror (work15) subscribes to the lVM WebSocket datastream and republishes
// each chain event as a chain.<kind> domain event on the Kafka bus, so application services consume
// on-chain events through one transport. It is non-custodial and read-only with respect to the chain
// (it never submits transactions); the chain RPC remains the authoritative source of truth.
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
	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/chain-event-mirror/internal/chain/wsstream"
	"github.com/paylink/chain-event-mirror/internal/config"
	"github.com/paylink/chain-event-mirror/internal/logging"
	"github.com/paylink/chain-event-mirror/internal/metrics"
	"github.com/paylink/chain-event-mirror/internal/mirror"
	"github.com/paylink/chain-event-mirror/internal/server"
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

	// work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set; never fatal.
	// The mirror is a bus producer, so its publish spans seed each chain.<kind> event's trace.
	otelShutdown, err := telemetry.Init(ctx, config.ServiceName, "0.1.0")
	if err != nil {
		log.Warn("telemetry init failed; tracing disabled", "err", err.Error())
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	pub, err := eventbus.NewPublisher(
		eventbus.Config{Brokers: eventbus.SplitBrokers(cfg.KafkaBrokers), ClientID: cfg.KafkaClientID},
		config.ServiceName, log,
	)
	if err != nil {
		log.Error("kafka publisher init failed", "err", err.Error())
		os.Exit(1)
	}
	defer pub.Close()

	m := metrics.New()
	mr := mirror.New(pub, m, log)
	src := wsstream.New(cfg.ChainWSURL, cfg.EventKinds, log)

	ready := []server.ReadyCheck{{Name: "kafka", Check: pub.Ping}}
	srv := server.New(m, ready)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// Chain mirror loop: WS datastream → chain.<kind> on the bus. Reconnects internally.
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("chain mirror starting", "ws", cfg.ChainWSURL, "kinds", cfg.EventKinds)
		if err := src.Run(ctx, mr.Handle); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("chain mirror stopped", "err", err.Error())
			stop()
		}
	}()

	// Internal HTTP surface (health/readiness/metrics).
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
