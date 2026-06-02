package eventbus

import (
	"os"
	"strings"
)

// Config configures a Publisher or a Consumer.
type Config struct {
	Brokers  []string // Kafka seed brokers (host:port)
	ClientID string   // client id reported to the broker
	GroupID  string   // consumer group (consumer only)
}

// ConfigFromEnv reads KAFKA_BROKERS (comma-separated), KAFKA_CLIENT_ID (default "linkmint"), and
// KAFKA_CONSUMER_GROUP. Per the 12-factor standard, all transport config comes from the env.
func ConfigFromEnv() Config {
	return Config{
		Brokers:  SplitBrokers(os.Getenv("KAFKA_BROKERS")),
		ClientID: envOr("KAFKA_CLIENT_ID", "linkmint"),
		GroupID:  os.Getenv("KAFKA_CONSUMER_GROUP"),
	}
}

// SplitBrokers parses a comma-separated broker list, trimming spaces and dropping empties.
func SplitBrokers(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
