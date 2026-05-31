// Package config loads service configuration from the environment only (12-factor).
// No hosts, ports, or secrets are ever hard-coded; every value has an env override.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ServiceName is the logical name used in logs, metrics, and idempotency namespacing.
const ServiceName = "proof-validator"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr    string // PROOF_VALIDATOR_HTTP_ADDR
	DatabaseURL string // PROOF_VALIDATOR_DATABASE_URL (postgres DSN)
	RedisURL    string // PROOF_VALIDATOR_REDIS_URL
	ChainRPCURL string // PROOF_VALIDATOR_CHAIN_RPC_URL (lVM JSON-RPC)

	// Proof verification.
	TrustedPubKeys []string // PROOF_VALIDATOR_TRUSTED_PUBKEYS (comma-separated uncompressed P-256 hex)

	// Settlement signing (the validator's own key).
	SignerMode     string // PROOF_VALIDATOR_SIGNER_MODE: service_key|unsigned
	ChainSignerKey string // PROOF_VALIDATOR_CHAIN_SIGNER_KEY (P-256 D scalar hex)

	// On-chain cross-check before broadcasting.
	PayLinkCrossCheck bool // PROOF_VALIDATOR_PAYLINK_CROSSCHECK

	// Devnet-only auto-stake bootstrap so the signer becomes an active validator.
	AutoStake             bool          // PROOF_VALIDATOR_AUTO_STAKE
	StakeAmount           uint64        // PROOF_VALIDATOR_STAKE_AMOUNT (0 = use chain minimumStake)
	AutoStakeTimeout      time.Duration // PROOF_VALIDATOR_AUTO_STAKE_TIMEOUT_SECONDS
	AutoStakePollInterval time.Duration // PROOF_VALIDATOR_AUTO_STAKE_POLL_MS

	LogLevel       string        // PROOF_VALIDATOR_LOG_LEVEL
	IdempotencyTTL time.Duration // PROOF_VALIDATOR_IDEMPOTENCY_TTL_SECONDS
	HTTPTimeout    time.Duration // PROOF_VALIDATOR_HTTP_TIMEOUT_SECONDS
}

// Load reads configuration from the environment, applying defaults that make a local `go run`
// work against the docker-compose network names. AutoStake defaults to false (production: the
// validator is staked out-of-band); docker-compose turns it on for the devnet.
func Load() (Config, error) {
	ttl, err := envSeconds("PROOF_VALIDATOR_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	httpTimeout, err := envSeconds("PROOF_VALIDATOR_HTTP_TIMEOUT_SECONDS", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	crossCheck, err := envBool("PROOF_VALIDATOR_PAYLINK_CROSSCHECK", true)
	if err != nil {
		return Config{}, err
	}
	autoStake, err := envBool("PROOF_VALIDATOR_AUTO_STAKE", false)
	if err != nil {
		return Config{}, err
	}
	stakeAmount, err := envUint64("PROOF_VALIDATOR_STAKE_AMOUNT", 0)
	if err != nil {
		return Config{}, err
	}
	autoStakeTimeout, err := envSeconds("PROOF_VALIDATOR_AUTO_STAKE_TIMEOUT_SECONDS", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	pollMS, err := envInt("PROOF_VALIDATOR_AUTO_STAKE_POLL_MS", 1000)
	if err != nil {
		return Config{}, err
	}

	signerMode := strings.ToLower(env("PROOF_VALIDATOR_SIGNER_MODE", "service_key"))
	if signerMode != "service_key" && signerMode != "unsigned" {
		return Config{}, fmt.Errorf("config: PROOF_VALIDATOR_SIGNER_MODE must be service_key|unsigned, got %q", signerMode)
	}

	return Config{
		HTTPAddr:              env("PROOF_VALIDATOR_HTTP_ADDR", ":8081"),
		DatabaseURL:           env("PROOF_VALIDATOR_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		RedisURL:              env("PROOF_VALIDATOR_REDIS_URL", "redis://localhost:6379/0"),
		ChainRPCURL:           env("PROOF_VALIDATOR_CHAIN_RPC_URL", "http://localhost:8545/"),
		TrustedPubKeys:        envCSV("PROOF_VALIDATOR_TRUSTED_PUBKEYS"),
		SignerMode:            signerMode,
		ChainSignerKey:        env("PROOF_VALIDATOR_CHAIN_SIGNER_KEY", ""),
		PayLinkCrossCheck:     crossCheck,
		AutoStake:             autoStake,
		StakeAmount:           stakeAmount,
		AutoStakeTimeout:      autoStakeTimeout,
		AutoStakePollInterval: time.Duration(pollMS) * time.Millisecond,
		LogLevel:              env("PROOF_VALIDATOR_LOG_LEVEL", "info"),
		IdempotencyTTL:        ttl,
		HTTPTimeout:           httpTimeout,
	}, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// envCSV parses a comma-separated env var into a trimmed, non-empty slice (nil when unset).
func envCSV(key string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envSeconds(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer (seconds), got %q", key, v)
	}
	return time.Duration(n) * time.Second, nil
}

func envInt(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer, got %q", key, v)
	}
	return n, nil
}

func envUint64(key string, def uint64) (uint64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a non-negative integer, got %q", key, v)
	}
	return n, nil
}

func envBool(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s must be a boolean, got %q", key, v)
	}
	return b, nil
}
