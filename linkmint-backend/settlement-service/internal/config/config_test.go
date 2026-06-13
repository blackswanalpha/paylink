package config

import (
	"math/big"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8101" {
		t.Errorf("HTTPAddr=%q, want :8101", cfg.HTTPAddr)
	}
	if cfg.Currency != "KES" {
		t.Errorf("Currency=%q, want KES", cfg.Currency)
	}
	if cfg.ScheduleInterval.Seconds() != 30 {
		t.Errorf("ScheduleInterval=%v, want 30s", cfg.ScheduleInterval)
	}
	if !cfg.EventConsumerEnabled || !cfg.ScheduleEnabled {
		t.Error("consumer/schedule should default enabled")
	}
}

func TestLoadMinPayout(t *testing.T) {
	t.Setenv("SETTLEMENT_MIN_PAYOUT", "KES:10000;usd:500")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MinPayoutFor("KES").Cmp(big.NewInt(10000)) != 0 {
		t.Errorf("KES min=%s, want 10000", cfg.MinPayoutFor("KES"))
	}
	if cfg.MinPayoutFor("USD").Cmp(big.NewInt(500)) != 0 {
		t.Errorf("USD min=%s, want 500 (case-insensitive)", cfg.MinPayoutFor("USD"))
	}
	if cfg.MinPayoutFor("GBP").Sign() != 0 {
		t.Errorf("unknown currency min=%s, want 0", cfg.MinPayoutFor("GBP"))
	}
}

func TestLoadMinPayoutInvalid(t *testing.T) {
	t.Setenv("SETTLEMENT_MIN_PAYOUT", "KES")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for malformed SETTLEMENT_MIN_PAYOUT")
	}
}

func TestLoadPlatformFeeBpsInvalid(t *testing.T) {
	t.Setenv("SETTLEMENT_PLATFORM_FEE_BPS", "10000")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for out-of-range SETTLEMENT_PLATFORM_FEE_BPS")
	}
}

func TestLoadBadInterval(t *testing.T) {
	t.Setenv("SETTLEMENT_SCHEDULE_INTERVAL_SECONDS", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric interval")
	}
}
