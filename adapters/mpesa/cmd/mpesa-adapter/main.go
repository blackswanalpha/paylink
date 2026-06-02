// Command mpesa-adapter runs the MPesa adapter CORE (work04): it receives a rail-neutral STK
// callback (from the Node Daraja rail service), normalizes it to the rail-agnostic proof (A.4),
// signs it (byte-exact via paylink-chain/pkg/lvm), and broadcasts it to the proof-validator
// (work03). It is non-custodial (A.1) — it holds no funds and only proves a payment happened; the
// chain decides settlement finality (A.3). MPesa/Daraja specifics live in the separate Node rail
// service (adapters/mpesa/daraja-service, see ADR-007).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	idempotency "github.com/paylink/idempotency-go"
	telemetry "github.com/paylink/telemetry-go"

	"github.com/paylink/mpesa-adapter/internal/broadcast"
	"github.com/paylink/mpesa-adapter/internal/config"
	"github.com/paylink/mpesa-adapter/internal/correlation"
	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/domain"
	"github.com/paylink/mpesa-adapter/internal/logging"
	"github.com/paylink/mpesa-adapter/internal/metrics"
	"github.com/paylink/mpesa-adapter/internal/redisx"
	"github.com/paylink/mpesa-adapter/internal/server"
	"github.com/paylink/mpesa-adapter/internal/signer"
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
	otelShutdown, err := telemetry.Init(ctx, config.ServiceName, "0.1.0")
	if err != nil {
		log.Warn("telemetry init failed; tracing disabled", "err", err.Error())
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	// One Redis connection, two consumers: the Idempotency-Key store and the correlation store.
	rc, err := redisx.New(cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer rc.Close()
	idem := idempotency.New(rc, config.ServiceName, cfg.IdempotencyTTL)
	corr := correlation.NewRedis(rc, cfg.CorrelationTTL)

	// The adapter's proof-signing key. A generated (ephemeral) key won't be in the validator's
	// trusted set, so warn loudly — it's a devnet convenience only.
	sgnr, generated, err := signer.Load(cfg.SignerKey)
	if err != nil {
		log.Error("signer init failed", "err", err.Error())
		os.Exit(1)
	}
	if generated {
		log.Warn("no MPESA_ADAPTER_SIGNER_KEY set — generated an ephemeral key; the validator will reject its proofs",
			"address", sgnr.Address().Hex(), "pubkey", sgnr.PubKeyHex())
	} else {
		log.Info("proof signer loaded", "address", sgnr.Address().Hex(), "pubkey", sgnr.PubKeyHex())
	}
	if cfg.InternalToken == "" {
		log.Warn("no MPESA_ADAPTER_INTERNAL_TOKEN set — the rail→core callback endpoint is unauthenticated (dev only)")
	}

	hc := &http.Client{Timeout: cfg.HTTPTimeout, Transport: telemetry.WrapTransport(http.DefaultTransport)}
	rail := daraja.NewHTTPClient(cfg.DarajaServiceURL, cfg.InternalToken, hc)
	bcast := broadcast.NewClient(cfg.ProofValidatorURL, hc)
	m := metrics.New()

	svc := domain.NewService(rail, corr, sgnr, bcast, cfg.DefaultShortCode, log, domain.WithMetrics(m))

	ready := []server.ReadyCheck{
		{Name: "redis", Check: idem.Ping},
		{Name: "daraja_service", Check: httpPinger(hc, strings.TrimRight(cfg.DarajaServiceURL, "/")+"/healthz")},
		{Name: "proof_validator", Check: httpPinger(hc, strings.TrimRight(cfg.ProofValidatorURL, "/")+"/internal/healthz")},
	}
	srv := server.New(svc, idem, m, log, cfg.InternalToken, ready)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "rail", cfg.DarajaServiceURL, "validator", cfg.ProofValidatorURL)
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

	select {
	case <-srvErr:
		os.Exit(1)
	default:
	}
}

// httpPinger returns a readiness probe that GETs url and expects a 2xx.
func httpPinger(hc *http.Client, url string) func(context.Context) error {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := hc.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return errStatus(resp.StatusCode)
		}
		return nil
	}
}

type errStatus int

func (e errStatus) Error() string { return "unhealthy: http " + strconv.Itoa(int(e)) }
