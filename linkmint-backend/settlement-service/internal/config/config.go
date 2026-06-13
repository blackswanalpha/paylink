// Package config loads service configuration from the environment only (12-factor).
// No hosts, ports, or secrets are ever hard-coded; every value has an env override.
package config

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// ServiceName is the logical name used in logs, metrics, idempotency namespacing, and as the
// event source.
const ServiceName = "settlement-service"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr             string              // SETTLEMENT_HTTP_ADDR
	DatabaseURL          string              // SETTLEMENT_DATABASE_URL (postgres DSN)
	RedisURL             string              // SETTLEMENT_REDIS_URL
	LogLevel             string              // SETTLEMENT_LOG_LEVEL (debug|info|warn|error)
	IdempotencyTTL       time.Duration       // SETTLEMENT_IDEMPOTENCY_TTL_SECONDS
	EventPublisherMode   string              // SETTLEMENT_EVENT_PUBLISHER_MODE — "log" (default) | "kafka"
	EventConsumerEnabled bool                // SETTLEMENT_EVENT_CONSUMER_ENABLED — toggle the bus consumers
	KafkaBrokers         string              // KAFKA_BROKERS — shared bus brokers (comma-separated)
	ScheduleEnabled      bool                // SETTLEMENT_SCHEDULE_ENABLED — toggle the payout scheduler
	ScheduleInterval     time.Duration       // SETTLEMENT_SCHEDULE_INTERVAL_SECONDS
	Currency             string              // SETTLEMENT_CURRENCY — single settlement currency (Phase 2)
	DefaultTZ            string              // SETTLEMENT_DEFAULT_TZ — cutoff tz when merchant tz unknown
	DefaultRail          string              // SETTLEMENT_DEFAULT_RAIL — payout rail when merchant rail unknown
	MinPayout            map[string]*big.Int // SETTLEMENT_MIN_PAYOUT — per-currency minimum net payout
	PlatformFeeBps       int64               // SETTLEMENT_PLATFORM_FEE_BPS — optional platform fee (A.5)
	IngestToken          string              // SETTLEMENT_INGEST_TOKEN — guards the rail-file ingest route
	DevCreatorAddr       string              // SETTLEMENT_DEV_CREATOR_ADDR — dev fallback for X-Creator-Addr
}

// Load reads configuration from the environment, applying defaults that make a local
// `go run` work against the docker-compose network names. Required-for-production values
// (DatabaseURL, RedisURL) default to localhost DSNs; main validates connectivity at boot.
func Load() (Config, error) {
	ttl, err := envSeconds("SETTLEMENT_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	scheduleInterval, err := envSeconds("SETTLEMENT_SCHEDULE_INTERVAL_SECONDS", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	consumerEnabled, err := envBool("SETTLEMENT_EVENT_CONSUMER_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	scheduleEnabled, err := envBool("SETTLEMENT_SCHEDULE_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	minPayout, err := parseMinPayout(os.Getenv("SETTLEMENT_MIN_PAYOUT"))
	if err != nil {
		return Config{}, err
	}
	feeBps, err := envInt("SETTLEMENT_PLATFORM_FEE_BPS", 0)
	if err != nil {
		return Config{}, err
	}
	if feeBps < 0 || feeBps >= 10_000 {
		return Config{}, fmt.Errorf("config: SETTLEMENT_PLATFORM_FEE_BPS must be in [0,10000), got %d", feeBps)
	}

	return Config{
		HTTPAddr:             env("SETTLEMENT_HTTP_ADDR", ":8101"),
		DatabaseURL:          env("SETTLEMENT_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		RedisURL:             env("SETTLEMENT_REDIS_URL", "redis://localhost:6379/0"),
		LogLevel:             env("SETTLEMENT_LOG_LEVEL", "info"),
		IdempotencyTTL:       ttl,
		EventPublisherMode:   env("SETTLEMENT_EVENT_PUBLISHER_MODE", "log"),
		EventConsumerEnabled: consumerEnabled,
		KafkaBrokers:         os.Getenv("KAFKA_BROKERS"),
		ScheduleEnabled:      scheduleEnabled,
		ScheduleInterval:     scheduleInterval,
		Currency:             strings.ToUpper(env("SETTLEMENT_CURRENCY", "KES")),
		DefaultTZ:            env("SETTLEMENT_DEFAULT_TZ", "Africa/Nairobi"),
		DefaultRail:          env("SETTLEMENT_DEFAULT_RAIL", "mpesa"),
		MinPayout:            minPayout,
		PlatformFeeBps:       feeBps,
		IngestToken:          os.Getenv("SETTLEMENT_INGEST_TOKEN"),
		DevCreatorAddr:       os.Getenv("SETTLEMENT_DEV_CREATOR_ADDR"),
	}, nil
}

// MinPayoutFor returns the configured minimum net payout for a currency, or zero (no minimum).
func (c Config) MinPayoutFor(currency string) *big.Int {
	if v, ok := c.MinPayout[strings.ToUpper(currency)]; ok {
		return v
	}
	return big.NewInt(0)
}

// parseMinPayout parses "CCY:minor;CCY:minor" into a per-currency map of minimum payouts.
func parseMinPayout(raw string) (map[string]*big.Int, error) {
	out := map[string]*big.Int{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("config: SETTLEMENT_MIN_PAYOUT entry %q must be CCY:minor", part)
		}
		ccy := strings.ToUpper(strings.TrimSpace(kv[0]))
		amt, ok := new(big.Int).SetString(strings.TrimSpace(kv[1]), 10)
		if ccy == "" || !ok || amt.Sign() < 0 {
			return nil, fmt.Errorf("config: SETTLEMENT_MIN_PAYOUT entry %q must be CCY:minor (non-negative integer)", part)
		}
		out[ccy] = amt
	}
	return out, nil
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

func envInt(key string, def int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, v)
	}
	return n, nil
}
