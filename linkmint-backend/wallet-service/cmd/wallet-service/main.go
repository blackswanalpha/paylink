// Command wallet-service runs the wallet-service (work24): a read-side surface over on-chain PLN.
// It indexes chain.* events (transfer/stake/unstake/reward/fee/burn) into the `wallet` schema and
// serves balances (read-through cached from the chain RPC), transaction history, staking
// positions/rewards, and public treasury stats. It also builds UNSIGNED staking-intent transactions.
//
// NON-CUSTODIAL (A.1): the service never holds private keys or funds — staking intents are returned
// unsigned for the client to sign. The build→sign→broadcast send path is out of scope (work34).
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

	"github.com/paylink/wallet-service/internal/chainrpc"
	"github.com/paylink/wallet-service/internal/config"
	"github.com/paylink/wallet-service/internal/consumer"
	"github.com/paylink/wallet-service/internal/domain"
	"github.com/paylink/wallet-service/internal/logging"
	"github.com/paylink/wallet-service/internal/metrics"
	"github.com/paylink/wallet-service/internal/server"
	pgstore "github.com/paylink/wallet-service/internal/store/postgres"
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
		log.Warn("WALLET_DEV_CREATOR_ADDR is set — dev-only X-Creator-Addr fallback active")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set; never fatal.
	otelShutdown, err := telemetry.Init(ctx, config.ServiceName, "0.1.0")
	if err != nil {
		log.Warn("telemetry init failed; tracing disabled", "err", err.Error())
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	// PostgreSQL store + migrations (wallet schema only — no shared ledger; read-side records no flows).
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

	// lVM JSON-RPC client (read-through for live balance/nonce + the unsigned-intent nonce/chain-id).
	hc := &http.Client{Timeout: cfg.ChainHTTPTimeout, Transport: telemetry.WrapTransport(http.DefaultTransport)}
	chainClient := chainrpc.NewClient(cfg.ChainRPCURL, hc)

	m := metrics.New()

	svc := domain.NewService(pg, chainClient, log,
		domain.WithMetrics(m),
		domain.WithChainID(cfg.ChainID),
		domain.WithBalanceCacheTTL(cfg.BalanceCacheTTL),
	)

	// postgres is the only HARD readiness dep; the chain is soft (reported degraded).
	ready := []server.ReadyCheck{{Name: "postgres", Check: pg.Ping}}
	srv := server.New(svc, m, log, ready, chainClient.Ping, cfg.DevCreatorAddr)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	var wg sync.WaitGroup

	// Chain indexer (work15): consume the `chain` topic and project events into the read-side. Offsets
	// commit only after a clean handle; the store's DbDedupe absorbs at-least-once redeliveries.
	if cfg.EventConsumerEnabled {
		if cfg.KafkaBrokers == "" {
			log.Warn("event consumer enabled but KAFKA_BROKERS is empty — indexer skipped; the read-side will not populate")
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
					log.Error("chain indexer stopped", "err", err.Error())
				}
			}()
			log.Info("chain indexer started", "topics", consumer.Topics, "group", config.ServiceName)
		}
	} else {
		log.Warn("event consumer disabled (WALLET_EVENT_CONSUMER_ENABLED=false) — the read-side will not populate")
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
