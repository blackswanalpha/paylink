// Package config loads service configuration from the environment only (12-factor). No hosts,
// ports, or secrets are ever hard-coded; every value has an env override. Mirrors the work03
// proof-validator config conventions (env/envBool/envSeconds helpers, ServiceName const).
//
// Architecture (hybrid, see ADR-007): this Go service is the protocol-critical CORE — it
// normalizes, signs (byte-exact via pkg/lvm), and broadcasts proofs. The MPesa/Daraja wire
// integration (OAuth, STK push, raw callback parsing) lives in a separate Node.js rail service
// (adapters/mpesa/daraja-service). The core calls it to start a charge and receives rail-neutral
// callback fields back from it. Daraja credentials live in the Node service, NOT here.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ServiceName is the logical name used in logs, metrics, and idempotency namespacing.
const ServiceName = "mpesa-adapter"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr string // MPESA_ADAPTER_HTTP_ADDR
	RedisURL string // MPESA_ADAPTER_REDIS_URL

	// Downstream: the proof-validator (work03) we broadcast signed proofs to.
	ProofValidatorURL string // MPESA_ADAPTER_PROOF_VALIDATOR_URL

	// The Node.js Daraja rail service: it performs the actual MPesa OAuth + STK push and ingests
	// the raw Daraja callback, handing us rail-neutral fields.
	DarajaServiceURL string // MPESA_ADAPTER_DARAJA_SERVICE_URL

	// Shared secret authenticating the two internal hops (core→rail /stk and rail→core
	// /v1/callbacks/mpesa). Devnet uses a well-known value; production injects a real one.
	InternalToken string // MPESA_ADAPTER_INTERNAL_TOKEN

	// The adapter's own P-256 signing key (D scalar hex). Its public key must be in the
	// validator's PROOF_VALIDATOR_TRUSTED_PUBKEYS. Empty ⇒ generate an ephemeral key + warn
	// (devnet convenience only — the validator would then reject every proof).
	SignerKey string // MPESA_ADAPTER_SIGNER_KEY

	// Default receiver shortcode used when /v1/charges omits one. A.1: the per-charge value is the
	// RECEIVER's shortcode; there is no LinkMint-owned collection account.
	DefaultShortCode string // MPESA_ADAPTER_DEFAULT_SHORTCODE

	CorrelationTTL time.Duration // MPESA_ADAPTER_CORRELATION_TTL_SECONDS (≈ PayLink expiry)
	IdempotencyTTL time.Duration // MPESA_ADAPTER_IDEMPOTENCY_TTL_SECONDS
	HTTPTimeout    time.Duration // MPESA_ADAPTER_HTTP_TIMEOUT_SECONDS (outbound rail + validator)
	LogLevel       string        // MPESA_ADAPTER_LOG_LEVEL
}

// Load reads configuration from the environment, applying defaults that make a local `go run`
// work against the docker-compose network names.
func Load() (Config, error) {
	corrTTL, err := envSeconds("MPESA_ADAPTER_CORRELATION_TTL_SECONDS", time.Hour)
	if err != nil {
		return Config{}, err
	}
	idemTTL, err := envSeconds("MPESA_ADAPTER_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	httpTimeout, err := envSeconds("MPESA_ADAPTER_HTTP_TIMEOUT_SECONDS", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:          env("MPESA_ADAPTER_HTTP_ADDR", ":8082"),
		RedisURL:          env("MPESA_ADAPTER_REDIS_URL", "redis://localhost:6379/0"),
		ProofValidatorURL: env("MPESA_ADAPTER_PROOF_VALIDATOR_URL", "http://localhost:8081"),
		DarajaServiceURL:  env("MPESA_ADAPTER_DARAJA_SERVICE_URL", "http://localhost:8083"),
		InternalToken:     env("MPESA_ADAPTER_INTERNAL_TOKEN", ""),
		SignerKey:         env("MPESA_ADAPTER_SIGNER_KEY", ""),
		DefaultShortCode:  env("MPESA_ADAPTER_DEFAULT_SHORTCODE", "174379"),
		CorrelationTTL:    corrTTL,
		IdempotencyTTL:    idemTTL,
		HTTPTimeout:       httpTimeout,
		LogLevel:          env("MPESA_ADAPTER_LOG_LEVEL", "info"),
	}, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envSeconds(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer (seconds), got %q", key, strings.TrimSpace(v))
	}
	return time.Duration(n) * time.Second, nil
}
