package config_test

import (
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// No env set (t.Setenv would set; here we rely on the test env being clean for these keys).
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8082" {
		t.Errorf("HTTPAddr = %q, want :8082", cfg.HTTPAddr)
	}
	if cfg.DefaultShortCode != "174379" {
		t.Errorf("DefaultShortCode = %q, want 174379", cfg.DefaultShortCode)
	}
	if cfg.CorrelationTTL != time.Hour {
		t.Errorf("CorrelationTTL = %s, want 1h", cfg.CorrelationTTL)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Errorf("IdempotencyTTL = %s, want 24h", cfg.IdempotencyTTL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("MPESA_ADAPTER_HTTP_ADDR", ":9999")
	t.Setenv("MPESA_ADAPTER_DARAJA_SERVICE_URL", "http://rail:8083")
	t.Setenv("MPESA_ADAPTER_SIGNER_KEY", "deadbeef")
	t.Setenv("MPESA_ADAPTER_CORRELATION_TTL_SECONDS", "120")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.DarajaServiceURL != "http://rail:8083" {
		t.Errorf("DarajaServiceURL = %q", cfg.DarajaServiceURL)
	}
	if cfg.SignerKey != "deadbeef" {
		t.Errorf("SignerKey = %q", cfg.SignerKey)
	}
	if cfg.CorrelationTTL != 120*time.Second {
		t.Errorf("CorrelationTTL = %s, want 2m", cfg.CorrelationTTL)
	}
}

func TestLoad_BadSeconds(t *testing.T) {
	t.Setenv("MPESA_ADAPTER_HTTP_TIMEOUT_SECONDS", "not-a-number")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for non-numeric seconds")
	}
}
