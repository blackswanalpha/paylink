// Command proof-validator runs the proof-validator service (work03): the off-chain bridge that
// verifies a signed, rail-agnostic payment proof and broadcasts a TxSubmitValidation settlement
// transaction to the lVM. It is non-custodial (A.1), defers settlement finality to the chain's
// quorum (A.3), accepts only the normalized proof shape (A.4), and never re-broadcasts a settled
// proof (A.7).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paylink/proof-validator/internal/autostake"
	"github.com/paylink/proof-validator/internal/chain"
	"github.com/paylink/proof-validator/internal/config"
	"github.com/paylink/proof-validator/internal/domain"
	"github.com/paylink/proof-validator/internal/events"
	"github.com/paylink/proof-validator/internal/idempotency"
	"github.com/paylink/proof-validator/internal/logging"
	"github.com/paylink/proof-validator/internal/metrics"
	"github.com/paylink/proof-validator/internal/proof"
	"github.com/paylink/proof-validator/internal/server"
	"github.com/paylink/proof-validator/internal/signer"
	pgstore "github.com/paylink/proof-validator/internal/store/postgres"
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

	// Redis-backed idempotency.
	rc, err := idempotency.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer rc.Close()
	idem := idempotency.New(rc, cfg.IdempotencyTTL)

	// Signer (the validator's own key) + proof verifier (trusted adapter keys).
	sgnr, generated, err := signer.Build(cfg.SignerMode, cfg.ChainSignerKey)
	if err != nil {
		log.Error("signer init failed", "err", err.Error())
		os.Exit(1)
	}
	if generated {
		log.Warn("no signer key configured — generated an ephemeral key (devnet only)", "address", sgnr.Address().Hex())
	}
	verifier, err := proof.NewVerifier(cfg.TrustedPubKeys)
	if err != nil {
		log.Error("proof verifier init failed", "err", err.Error())
		os.Exit(1)
	}
	if verifier.TrustedCount() == 0 {
		log.Warn("no trusted proof pubkeys configured — all proofs will be rejected (set PROOF_VALIDATOR_TRUSTED_PUBKEYS)")
	}

	// Outbound chain client + nonce manager + event publisher + metrics.
	hc := &http.Client{Timeout: cfg.HTTPTimeout}
	chainClient := chain.NewClient(cfg.ChainRPCURL, hc)
	nonce := chain.NewNonceManager(chainClient)
	publisher := events.NewLogPublisher(log)
	m := metrics.New()

	svc := domain.NewService(pg, chainClient, verifier, sgnr, nonce, publisher, log,
		domain.WithMetrics(m), domain.WithCrossCheck(cfg.PayLinkCrossCheck))

	// Devnet auto-stake: make the signer an active validator so a single-validator devnet can
	// reach quorum and settle. Blocks until active so we never serve before we can settle.
	if cfg.AutoStake {
		bs := autostake.New(chainClient, sgnr, nonce, log, cfg.StakeAmount, cfg.AutoStakePollInterval, cfg.AutoStakeTimeout)
		if err := bs.EnsureActive(ctx); err != nil {
			log.Error("auto-stake failed", "err", err.Error())
			os.Exit(1)
		}
	}

	selfAddr := sgnr.Address().Hex()
	ready := []server.ReadyCheck{
		{Name: "postgres", Check: pg.Ping},
		{Name: "redis", Check: idem.Ping},
		{Name: "chain", Check: chainClient.Ping},
		{Name: "validator_active", Check: func(ctx context.Context) error {
			v, found, err := chainClient.GetValidator(ctx, selfAddr)
			if err != nil {
				return err
			}
			if !found || !v.IsActive {
				return fmt.Errorf("validator %s is not active", selfAddr)
			}
			return nil
		}},
	}
	srv := server.New(svc, idem, m, log, ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// HTTP server. A bind/serve failure triggers shutdown and a non-zero exit (below).
	srvErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "validator", selfAddr)
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
	log.Info("stopped")

	// Exit non-zero if the server died on a listen/serve error (so probes/orchestrators notice).
	select {
	case <-srvErr:
		os.Exit(1)
	default:
	}
}
