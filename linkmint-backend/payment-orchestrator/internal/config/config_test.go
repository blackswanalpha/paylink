package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Ensure a clean env for the keys we read.
	for _, k := range []string{
		"PAYMENT_HTTP_ADDR", "PAYMENT_DATABASE_URL", "PAYMENT_REDIS_URL", "PAYMENT_CHAIN_RPC_URL",
		"PAYMENT_CHAIN_WS_URL", "PAYMENT_PAYLINK_SERVICE_URL", "PAYMENT_ADAPTER_MPESA_URL", "PAYMENT_LOG_LEVEL",
		"PAYMENT_IDEMPOTENCY_TTL_SECONDS", "PAYMENT_CHAIN_EVENTS_ENABLED", "PAYMENT_HTTP_TIMEOUT_SECONDS",
	} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q", cfg.HTTPAddr)
	}
	if cfg.AdapterMpesaURL != "" {
		t.Errorf("AdapterMpesaURL default = %q, want empty", cfg.AdapterMpesaURL)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Errorf("IdempotencyTTL default = %v", cfg.IdempotencyTTL)
	}
	if !cfg.ChainEventsEnabled {
		t.Error("ChainEventsEnabled default should be true")
	}
	if cfg.HTTPTimeout != 10*time.Second {
		t.Errorf("HTTPTimeout default = %v", cfg.HTTPTimeout)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PAYMENT_HTTP_ADDR", ":9999")
	t.Setenv("PAYMENT_PAYLINK_SERVICE_URL", "http://paylinks:8000/")
	t.Setenv("PAYMENT_ADAPTER_MPESA_URL", "http://mpesa-adapter:8082/")
	t.Setenv("PAYMENT_IDEMPOTENCY_TTL_SECONDS", "3600")
	t.Setenv("PAYMENT_CHAIN_EVENTS_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.PayLinkServiceURL != "http://paylinks:8000" { // trailing slash trimmed
		t.Errorf("PayLinkServiceURL = %q", cfg.PayLinkServiceURL)
	}
	if cfg.AdapterMpesaURL != "http://mpesa-adapter:8082" { // trailing slash trimmed
		t.Errorf("AdapterMpesaURL = %q", cfg.AdapterMpesaURL)
	}
	if cfg.IdempotencyTTL != time.Hour {
		t.Errorf("IdempotencyTTL = %v", cfg.IdempotencyTTL)
	}
	if cfg.ChainEventsEnabled {
		t.Error("ChainEventsEnabled should be false")
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Setenv("PAYMENT_IDEMPOTENCY_TTL_SECONDS", "not-a-number")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid TTL")
	}
	t.Setenv("PAYMENT_IDEMPOTENCY_TTL_SECONDS", "3600")
	t.Setenv("PAYMENT_CHAIN_EVENTS_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Error("expected error for invalid bool")
	}
}
