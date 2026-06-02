// Package config loads the chain-event-mirror configuration from the environment (12-factor).
package config

import (
	"fmt"
	"os"
	"strings"
)

// ServiceName is the logical service name (stamped into logs and event envelopes as the source).
const ServiceName = "chain-event-mirror"

// Config is the mirror's runtime configuration.
type Config struct {
	HTTPAddr      string   // CEM_HTTP_ADDR — health/readiness/metrics listen address
	ChainWSURL    string   // CEM_CHAIN_WS_URL — lVM datastream /ws
	KafkaBrokers  string   // KAFKA_BROKERS — comma-separated seed brokers (shared lib env)
	KafkaClientID string   // KAFKA_CLIENT_ID
	EventKinds    []string // CEM_CHAIN_EVENT_KINDS — restrict mirrored kinds (empty = all)
	LogLevel      string   // CEM_LOG_LEVEL
}

// Load reads and validates the configuration.
func Load() (Config, error) {
	c := Config{
		HTTPAddr:      env("CEM_HTTP_ADDR", ":8096"),
		ChainWSURL:    env("CEM_CHAIN_WS_URL", "ws://localhost:8545/ws"),
		KafkaBrokers:  os.Getenv("KAFKA_BROKERS"),
		KafkaClientID: env("KAFKA_CLIENT_ID", ServiceName),
		EventKinds:    splitCSV(os.Getenv("CEM_CHAIN_EVENT_KINDS")),
		LogLevel:      env("CEM_LOG_LEVEL", "INFO"),
	}
	if c.ChainWSURL == "" {
		return Config{}, fmt.Errorf("CEM_CHAIN_WS_URL is required")
	}
	if c.KafkaBrokers == "" {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is required")
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
