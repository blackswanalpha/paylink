package config_test

import (
	"testing"
	"time"

	"github.com/paylink/proof-validator/internal/config"
)

// allKeys are every env var Load reads; defaults() sets them empty so a defaults test is
// deterministic regardless of the ambient environment.
var allKeys = []string{
	"PROOF_VALIDATOR_HTTP_ADDR", "PROOF_VALIDATOR_DATABASE_URL", "PROOF_VALIDATOR_REDIS_URL",
	"PROOF_VALIDATOR_CHAIN_RPC_URL", "PROOF_VALIDATOR_TRUSTED_PUBKEYS", "PROOF_VALIDATOR_SIGNER_MODE",
	"PROOF_VALIDATOR_CHAIN_SIGNER_KEY", "PROOF_VALIDATOR_PAYLINK_CROSSCHECK", "PROOF_VALIDATOR_AUTO_STAKE",
	"PROOF_VALIDATOR_STAKE_AMOUNT", "PROOF_VALIDATOR_AUTO_STAKE_TIMEOUT_SECONDS", "PROOF_VALIDATOR_AUTO_STAKE_POLL_MS",
	"PROOF_VALIDATOR_LOG_LEVEL", "PROOF_VALIDATOR_IDEMPOTENCY_TTL_SECONDS", "PROOF_VALIDATOR_HTTP_TIMEOUT_SECONDS",
}

func clearAll(t *testing.T) {
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearAll(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8081" {
		t.Errorf("HTTPAddr = %q, want :8081", cfg.HTTPAddr)
	}
	if cfg.SignerMode != "service_key" {
		t.Errorf("SignerMode = %q, want service_key", cfg.SignerMode)
	}
	if !cfg.PayLinkCrossCheck {
		t.Error("PayLinkCrossCheck should default true")
	}
	if cfg.AutoStake {
		t.Error("AutoStake should default false")
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Errorf("IdempotencyTTL = %v, want 24h", cfg.IdempotencyTTL)
	}
	if cfg.AutoStakePollInterval != time.Second {
		t.Errorf("AutoStakePollInterval = %v, want 1s", cfg.AutoStakePollInterval)
	}
	if len(cfg.TrustedPubKeys) != 0 {
		t.Errorf("TrustedPubKeys should be empty by default, got %v", cfg.TrustedPubKeys)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	clearAll(t)
	t.Setenv("PROOF_VALIDATOR_HTTP_ADDR", ":9999")
	t.Setenv("PROOF_VALIDATOR_AUTO_STAKE", "true")
	t.Setenv("PROOF_VALIDATOR_PAYLINK_CROSSCHECK", "false")
	t.Setenv("PROOF_VALIDATOR_STAKE_AMOUNT", "12345")
	t.Setenv("PROOF_VALIDATOR_TRUSTED_PUBKEYS", " a , b ,, c ")
	t.Setenv("PROOF_VALIDATOR_AUTO_STAKE_POLL_MS", "250")
	t.Setenv("PROOF_VALIDATOR_SIGNER_MODE", "UNSIGNED")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9999" || !cfg.AutoStake || cfg.PayLinkCrossCheck {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.StakeAmount != 12345 {
		t.Errorf("StakeAmount = %d, want 12345", cfg.StakeAmount)
	}
	if len(cfg.TrustedPubKeys) != 3 || cfg.TrustedPubKeys[0] != "a" || cfg.TrustedPubKeys[2] != "c" {
		t.Errorf("TrustedPubKeys = %v, want [a b c] trimmed", cfg.TrustedPubKeys)
	}
	if cfg.AutoStakePollInterval != 250*time.Millisecond {
		t.Errorf("poll interval = %v, want 250ms", cfg.AutoStakePollInterval)
	}
	if cfg.SignerMode != "unsigned" {
		t.Errorf("SignerMode = %q, want unsigned (lowercased)", cfg.SignerMode)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("bad signer mode", func(t *testing.T) {
		clearAll(t)
		t.Setenv("PROOF_VALIDATOR_SIGNER_MODE", "magic")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error for invalid signer mode")
		}
	})
	t.Run("bad bool", func(t *testing.T) {
		clearAll(t)
		t.Setenv("PROOF_VALIDATOR_AUTO_STAKE", "maybe")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error for invalid bool")
		}
	})
	t.Run("bad uint", func(t *testing.T) {
		clearAll(t)
		t.Setenv("PROOF_VALIDATOR_STAKE_AMOUNT", "-5")
		if _, err := config.Load(); err == nil {
			t.Fatal("expected error for invalid uint")
		}
	})
}
