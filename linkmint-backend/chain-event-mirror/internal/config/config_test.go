package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "redpanda:9092")
	t.Setenv("CEM_HTTP_ADDR", "")
	t.Setenv("CEM_CHAIN_EVENT_KINDS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPAddr != ":8096" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.ChainWSURL != "ws://localhost:8545/ws" {
		t.Errorf("ChainWSURL = %q", cfg.ChainWSURL)
	}
	if cfg.KafkaClientID != "chain-event-mirror" {
		t.Errorf("KafkaClientID = %q", cfg.KafkaClientID)
	}
	if len(cfg.EventKinds) != 0 {
		t.Errorf("EventKinds = %v (want empty = all)", cfg.EventKinds)
	}
}

func TestLoad_RequiresBrokers(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error when KAFKA_BROKERS is unset")
	}
}

func TestLoad_EventKindsCSV(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "b:9092")
	t.Setenv("CEM_CHAIN_EVENT_KINDS", "paylink.verified, paylink.failed ,")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.EventKinds) != 2 || cfg.EventKinds[0] != "paylink.verified" || cfg.EventKinds[1] != "paylink.failed" {
		t.Fatalf("EventKinds = %v", cfg.EventKinds)
	}
}
