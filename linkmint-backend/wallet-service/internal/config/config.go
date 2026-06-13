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

// ServiceName is the logical name used in logs, metrics, the consumer group, and as the
// event source.
const ServiceName = "wallet-service"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr             string        // WALLET_HTTP_ADDR
	DatabaseURL          string        // WALLET_DATABASE_URL (postgres DSN)
	LogLevel             string        // WALLET_LOG_LEVEL (debug|info|warn|error)
	ChainRPCURL          string        // WALLET_CHAIN_RPC_URL (lVM JSON-RPC base)
	ChainHTTPTimeout     time.Duration // WALLET_CHAIN_HTTP_TIMEOUT_SECONDS
	ChainID              string        // WALLET_CHAIN_ID (fallback; live id fetched lazily)
	EventConsumerEnabled bool          // WALLET_EVENT_CONSUMER_ENABLED — toggle the chain indexer
	KafkaBrokers         string        // KAFKA_BROKERS — shared bus brokers (comma-separated)
	BalanceCacheTTL      time.Duration // WALLET_BALANCE_CACHE_TTL_SECONDS — read-through cache window
	DevCreatorAddr       string        // WALLET_DEV_CREATOR_ADDR — dev fallback for X-Creator-Addr
}

// Load reads configuration from the environment, applying defaults that make a local `go run`
// work against the docker-compose network names. main validates DB connectivity at boot; the
// chain RPC is a soft dependency (the indexed read-side serves while it is down).
func Load() (Config, error) {
	chainTimeout, err := envSeconds("WALLET_CHAIN_HTTP_TIMEOUT_SECONDS", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	cacheTTL, err := envSeconds("WALLET_BALANCE_CACHE_TTL_SECONDS", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	consumerEnabled, err := envBool("WALLET_EVENT_CONSUMER_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:             env("WALLET_HTTP_ADDR", ":8102"),
		DatabaseURL:          env("WALLET_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		LogLevel:             env("WALLET_LOG_LEVEL", "info"),
		ChainRPCURL:          env("WALLET_CHAIN_RPC_URL", "http://localhost:8545/"),
		ChainHTTPTimeout:     chainTimeout,
		ChainID:              env("WALLET_CHAIN_ID", "paylink-devnet"),
		EventConsumerEnabled: consumerEnabled,
		KafkaBrokers:         os.Getenv("KAFKA_BROKERS"),
		BalanceCacheTTL:      cacheTTL,
		DevCreatorAddr:       strings.ToLower(os.Getenv("WALLET_DEV_CREATOR_ADDR")),
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
