// Package config loads service configuration from the environment only (12-factor).
// No hosts, ports, or secrets are ever hard-coded; every value has an env override.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServiceName is the logical name used in logs, metrics, and idempotency namespacing.
const ServiceName = "escrow-manager"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr             string        // ESCROW_HTTP_ADDR
	DatabaseURL          string        // ESCROW_DATABASE_URL (postgres DSN)
	RedisURL             string        // ESCROW_REDIS_URL
	LogLevel             string        // ESCROW_LOG_LEVEL (debug|info|warn|error)
	IdempotencyTTL       time.Duration // ESCROW_IDEMPOTENCY_TTL_SECONDS
	EventPublisherMode   string        // ESCROW_EVENT_PUBLISHER_MODE — "log" (default) | "kafka" (work15)
	EventConsumerEnabled bool          // ESCROW_EVENT_CONSUMER_ENABLED — toggle the chain.paylink.verified consumer
	KafkaBrokers         string        // KAFKA_BROKERS — shared bus brokers (comma-separated)
	SweepEnabled         bool          // ESCROW_SWEEP_ENABLED — toggle the release/timeout sweeper
	SweepInterval        time.Duration // ESCROW_SWEEP_INTERVAL_SECONDS
	DefaultTimeout       time.Duration // ESCROW_DEFAULT_TIMEOUT_SECONDS — timeout_at default on create
	DevCreatorAddr       string        // ESCROW_DEV_CREATOR_ADDR — dev fallback for X-Creator-Addr ("" ⇒ 401)
}

// Load reads configuration from the environment, applying defaults that make a local
// `go run` work against the docker-compose network names. Required-for-production values
// (DatabaseURL, RedisURL) default to localhost DSNs; main validates connectivity at boot.
func Load() (Config, error) {
	ttl, err := envSeconds("ESCROW_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sweepInterval, err := envSeconds("ESCROW_SWEEP_INTERVAL_SECONDS", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	defaultTimeout, err := envSeconds("ESCROW_DEFAULT_TIMEOUT_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	consumerEnabled, err := envBool("ESCROW_EVENT_CONSUMER_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	sweepEnabled, err := envBool("ESCROW_SWEEP_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:             env("ESCROW_HTTP_ADDR", ":8098"),
		DatabaseURL:          env("ESCROW_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		RedisURL:             env("ESCROW_REDIS_URL", "redis://localhost:6379/0"),
		LogLevel:             env("ESCROW_LOG_LEVEL", "info"),
		IdempotencyTTL:       ttl,
		EventPublisherMode:   env("ESCROW_EVENT_PUBLISHER_MODE", "log"),
		EventConsumerEnabled: consumerEnabled,
		KafkaBrokers:         os.Getenv("KAFKA_BROKERS"),
		SweepEnabled:         sweepEnabled,
		SweepInterval:        sweepInterval,
		DefaultTimeout:       defaultTimeout,
		DevCreatorAddr:       os.Getenv("ESCROW_DEV_CREATOR_ADDR"),
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
		return 0, fmt.Errorf("config: %s must be a non-negative integer (seconds), got %q", key, v)
	}
	return time.Duration(n) * time.Second, nil
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
