package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPAddr != ":8094" {
		t.Fatalf("addr=%s", cfg.HTTPAddr)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Fatalf("ttl=%v", cfg.IdempotencyTTL)
	}
	if len(cfg.ReaderRoles) != 2 {
		t.Fatalf("default roles should be admin,compliance, got %v", cfg.ReaderRoles)
	}
	if cfg.IntakeEnabled {
		t.Fatal("intake should default to false")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("AUDIT_HTTP_ADDR", ":9000")
	t.Setenv("AUDIT_READER_ROLES", "admin, ops ,")
	t.Setenv("AUDIT_IDEMPOTENCY_TTL_SECONDS", "60")
	t.Setenv("AUDIT_INTAKE_ENABLED", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Fatal("http addr override failed")
	}
	if len(cfg.ReaderRoles) != 2 || cfg.ReaderRoles[0] != "admin" || cfg.ReaderRoles[1] != "ops" {
		t.Fatalf("roles parse failed: %v", cfg.ReaderRoles)
	}
	if cfg.IdempotencyTTL != 60*time.Second {
		t.Fatal("ttl override failed")
	}
	if !cfg.IntakeEnabled {
		t.Fatal("intake override failed")
	}
}

func TestExplicitEmptyReaderRoles(t *testing.T) {
	t.Setenv("AUDIT_READER_ROLES", "")
	cfg, _ := Load()
	if len(cfg.ReaderRoles) != 0 {
		t.Fatalf("set-but-empty should yield no roles, got %v", cfg.ReaderRoles)
	}
}

func TestLoadInvalid(t *testing.T) {
	t.Setenv("AUDIT_IDEMPOTENCY_TTL_SECONDS", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("negative ttl must error")
	}
	t.Setenv("AUDIT_IDEMPOTENCY_TTL_SECONDS", "86400")
	t.Setenv("AUDIT_INTAKE_ENABLED", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("bad bool must error")
	}
}
