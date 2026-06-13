package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{
		"WALLET_HTTP_ADDR", "WALLET_DATABASE_URL", "WALLET_LOG_LEVEL", "WALLET_CHAIN_RPC_URL",
		"WALLET_CHAIN_HTTP_TIMEOUT_SECONDS", "WALLET_CHAIN_ID", "WALLET_EVENT_CONSUMER_ENABLED",
		"KAFKA_BROKERS", "WALLET_BALANCE_CACHE_TTL_SECONDS", "WALLET_DEV_CREATOR_ADDR",
	} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8102" {
		t.Errorf("HTTPAddr = %q, want :8102", cfg.HTTPAddr)
	}
	if cfg.ChainRPCURL != "http://localhost:8545/" {
		t.Errorf("ChainRPCURL = %q", cfg.ChainRPCURL)
	}
	if cfg.ChainID != "paylink-devnet" {
		t.Errorf("ChainID = %q", cfg.ChainID)
	}
	if !cfg.EventConsumerEnabled {
		t.Error("EventConsumerEnabled should default true")
	}
	if cfg.ChainHTTPTimeout != 5*time.Second {
		t.Errorf("ChainHTTPTimeout = %v", cfg.ChainHTTPTimeout)
	}
	if cfg.BalanceCacheTTL != 5*time.Second {
		t.Errorf("BalanceCacheTTL = %v", cfg.BalanceCacheTTL)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("WALLET_HTTP_ADDR", ":9999")
	t.Setenv("WALLET_CHAIN_ID", "paylink-test")
	t.Setenv("WALLET_EVENT_CONSUMER_ENABLED", "false")
	t.Setenv("WALLET_BALANCE_CACHE_TTL_SECONDS", "30")
	t.Setenv("KAFKA_BROKERS", "redpanda:9092")
	t.Setenv("WALLET_DEV_CREATOR_ADDR", "0xABCDEF0000000000000000000000000000000001")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.ChainID != "paylink-test" {
		t.Errorf("ChainID = %q", cfg.ChainID)
	}
	if cfg.EventConsumerEnabled {
		t.Error("EventConsumerEnabled should be false")
	}
	if cfg.BalanceCacheTTL != 30*time.Second {
		t.Errorf("BalanceCacheTTL = %v", cfg.BalanceCacheTTL)
	}
	if cfg.KafkaBrokers != "redpanda:9092" {
		t.Errorf("KafkaBrokers = %q", cfg.KafkaBrokers)
	}
	// DevCreatorAddr is lower-cased.
	if cfg.DevCreatorAddr != "0xabcdef0000000000000000000000000000000001" {
		t.Errorf("DevCreatorAddr = %q (want lower-cased)", cfg.DevCreatorAddr)
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Setenv("WALLET_BALANCE_CACHE_TTL_SECONDS", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-integer TTL")
	}

	t.Setenv("WALLET_BALANCE_CACHE_TTL_SECONDS", "5")
	t.Setenv("WALLET_EVENT_CONSUMER_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-bool consumer flag")
	}
}
