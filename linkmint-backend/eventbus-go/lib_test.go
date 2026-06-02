package eventbus

import (
	"context"
	"testing"
)

func TestNewPublisher_RequiresBrokers(t *testing.T) {
	if _, err := NewPublisher(Config{}, "svc", nil); err == nil {
		t.Fatal("expected error with no brokers")
	}
}

func TestNewConsumer_Validation(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		topics []string
	}{
		{"no brokers", Config{GroupID: "g"}, []string{"paylink"}},
		{"no group", Config{Brokers: []string{"b:9092"}}, []string{"paylink"}},
		{"no topics", Config{Brokers: []string{"b:9092"}, GroupID: "g"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewConsumer(c.cfg, c.topics, nil); err == nil {
				t.Fatalf("%s: expected error", c.name)
			}
		})
	}
}

func TestWithCorrelationID_RoundTrip(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "trace-xyz")
	if got := correlationFrom(ctx); got != "trace-xyz" {
		t.Fatalf("correlationFrom = %q", got)
	}
	if got := correlationFrom(context.Background()); got != "" {
		t.Fatalf("empty context should yield empty correlation id, got %q", got)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "a:9092, b:9092")
	t.Setenv("KAFKA_CLIENT_ID", "svc-x")
	t.Setenv("KAFKA_CONSUMER_GROUP", "grp-x")
	cfg := ConfigFromEnv()
	if len(cfg.Brokers) != 2 || cfg.Brokers[0] != "a:9092" || cfg.Brokers[1] != "b:9092" {
		t.Fatalf("brokers = %v", cfg.Brokers)
	}
	if cfg.ClientID != "svc-x" || cfg.GroupID != "grp-x" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigFromEnv_ClientIDDefault(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "a:9092")
	t.Setenv("KAFKA_CLIENT_ID", "")
	if got := ConfigFromEnv().ClientID; got != "linkmint" {
		t.Fatalf("default client id = %q", got)
	}
}
