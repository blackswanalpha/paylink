package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Ensure a clean env for the keys we read.
	for _, k := range []string{
		"ESCROW_HTTP_ADDR", "ESCROW_DATABASE_URL", "ESCROW_REDIS_URL", "ESCROW_LOG_LEVEL",
		"ESCROW_IDEMPOTENCY_TTL_SECONDS", "ESCROW_EVENT_PUBLISHER_MODE", "ESCROW_EVENT_CONSUMER_ENABLED",
		"KAFKA_BROKERS", "ESCROW_SWEEP_ENABLED", "ESCROW_SWEEP_INTERVAL_SECONDS",
		"ESCROW_DEFAULT_TIMEOUT_SECONDS", "ESCROW_DEV_CREATOR_ADDR",
	} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8098" {
		t.Errorf("HTTPAddr default = %q", cfg.HTTPAddr)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Errorf("IdempotencyTTL default = %v", cfg.IdempotencyTTL)
	}
	if cfg.EventPublisherMode != "log" {
		t.Errorf("EventPublisherMode default = %q", cfg.EventPublisherMode)
	}
	if !cfg.EventConsumerEnabled {
		t.Error("EventConsumerEnabled default should be true")
	}
	if !cfg.SweepEnabled {
		t.Error("SweepEnabled default should be true")
	}
	if cfg.SweepInterval != 10*time.Second {
		t.Errorf("SweepInterval default = %v", cfg.SweepInterval)
	}
	if cfg.DefaultTimeout != 24*time.Hour {
		t.Errorf("DefaultTimeout default = %v", cfg.DefaultTimeout)
	}
	if cfg.DevCreatorAddr != "" {
		t.Errorf("DevCreatorAddr default = %q, want empty", cfg.DevCreatorAddr)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("ESCROW_HTTP_ADDR", ":9999")
	t.Setenv("ESCROW_IDEMPOTENCY_TTL_SECONDS", "3600")
	t.Setenv("ESCROW_EVENT_PUBLISHER_MODE", "kafka")
	t.Setenv("ESCROW_EVENT_CONSUMER_ENABLED", "false")
	t.Setenv("KAFKA_BROKERS", "redpanda:9092")
	t.Setenv("ESCROW_SWEEP_ENABLED", "false")
	t.Setenv("ESCROW_SWEEP_INTERVAL_SECONDS", "1")
	t.Setenv("ESCROW_DEFAULT_TIMEOUT_SECONDS", "60")
	t.Setenv("ESCROW_DEV_CREATOR_ADDR", "0xabc")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.IdempotencyTTL != time.Hour {
		t.Errorf("IdempotencyTTL = %v", cfg.IdempotencyTTL)
	}
	if cfg.EventPublisherMode != "kafka" || cfg.KafkaBrokers != "redpanda:9092" {
		t.Errorf("publisher mode/brokers = %q/%q", cfg.EventPublisherMode, cfg.KafkaBrokers)
	}
	if cfg.EventConsumerEnabled {
		t.Error("EventConsumerEnabled should be false")
	}
	if cfg.SweepEnabled {
		t.Error("SweepEnabled should be false")
	}
	if cfg.SweepInterval != time.Second {
		t.Errorf("SweepInterval = %v", cfg.SweepInterval)
	}
	if cfg.DefaultTimeout != time.Minute {
		t.Errorf("DefaultTimeout = %v", cfg.DefaultTimeout)
	}
	if cfg.DevCreatorAddr != "0xabc" {
		t.Errorf("DevCreatorAddr = %q", cfg.DevCreatorAddr)
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Setenv("ESCROW_IDEMPOTENCY_TTL_SECONDS", "not-a-number")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid TTL")
	}
	t.Setenv("ESCROW_IDEMPOTENCY_TTL_SECONDS", "3600")
	t.Setenv("ESCROW_SWEEP_INTERVAL_SECONDS", "-1")
	if _, err := Load(); err == nil {
		t.Error("expected error for negative interval")
	}
	t.Setenv("ESCROW_SWEEP_INTERVAL_SECONDS", "10")
	t.Setenv("ESCROW_DEFAULT_TIMEOUT_SECONDS", "nope")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid timeout")
	}
	t.Setenv("ESCROW_DEFAULT_TIMEOUT_SECONDS", "86400")
	t.Setenv("ESCROW_EVENT_CONSUMER_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid bool")
	}
	t.Setenv("ESCROW_EVENT_CONSUMER_ENABLED", "true")
	t.Setenv("ESCROW_SWEEP_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid sweep bool")
	}
}
