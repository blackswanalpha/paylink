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
const ServiceName = "payment-orchestrator"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr           string        // PAYMENT_HTTP_ADDR
	DatabaseURL        string        // PAYMENT_DATABASE_URL (postgres DSN)
	RedisURL           string        // PAYMENT_REDIS_URL
	ChainRPCURL        string        // PAYMENT_CHAIN_RPC_URL (lVM JSON-RPC)
	ChainWSURL         string        // PAYMENT_CHAIN_WS_URL  (lVM datastream /ws)
	PayLinkServiceURL  string        // PAYMENT_PAYLINK_SERVICE_URL
	AdapterMpesaURL    string        // PAYMENT_ADAPTER_MPESA_URL — registered MPesa adapter (work04); "" if none
	LogLevel           string        // PAYMENT_LOG_LEVEL (debug|info|warn|error)
	IdempotencyTTL     time.Duration // PAYMENT_IDEMPOTENCY_TTL_SECONDS
	ChainEventsEnabled bool          // PAYMENT_CHAIN_EVENTS_ENABLED — toggle the WS subscriber
	HTTPTimeout        time.Duration // PAYMENT_HTTP_TIMEOUT_SECONDS — outbound HTTP timeout
	EventPublisherMode string        // PAYMENT_EVENT_PUBLISHER_MODE — "log" (default) | "kafka" (work15)
	KafkaBrokers       string        // KAFKA_BROKERS — shared bus brokers (comma-separated); kafka mode
}

// Load reads configuration from the environment, applying defaults that make a local
// `go run` work against the docker-compose network names. Required-for-production values
// (DatabaseURL, RedisURL) default to localhost DSNs; main validates connectivity at boot.
func Load() (Config, error) {
	ttl, err := envSeconds("PAYMENT_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	httpTimeout, err := envSeconds("PAYMENT_HTTP_TIMEOUT_SECONDS", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	eventsEnabled, err := envBool("PAYMENT_CHAIN_EVENTS_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:           env("PAYMENT_HTTP_ADDR", ":8080"),
		DatabaseURL:        env("PAYMENT_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		RedisURL:           env("PAYMENT_REDIS_URL", "redis://localhost:6379/0"),
		ChainRPCURL:        env("PAYMENT_CHAIN_RPC_URL", "http://localhost:8545/"),
		ChainWSURL:         env("PAYMENT_CHAIN_WS_URL", "ws://localhost:8545/ws"),
		PayLinkServiceURL:  strings.TrimRight(env("PAYMENT_PAYLINK_SERVICE_URL", "http://localhost:8000"), "/"),
		AdapterMpesaURL:    strings.TrimRight(env("PAYMENT_ADAPTER_MPESA_URL", ""), "/"),
		LogLevel:           env("PAYMENT_LOG_LEVEL", "info"),
		IdempotencyTTL:     ttl,
		ChainEventsEnabled: eventsEnabled,
		HTTPTimeout:        httpTimeout,
		EventPublisherMode: env("PAYMENT_EVENT_PUBLISHER_MODE", "log"),
		KafkaBrokers:       os.Getenv("KAFKA_BROKERS"),
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
