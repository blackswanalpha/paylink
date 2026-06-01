// Package config loads service configuration from the environment only (12-factor).
// No hosts, ports, or secrets are ever hard-coded; every value has an env override.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ServiceName is the logical name used in logs, metrics, and idempotency namespacing.
const ServiceName = "audit-log-service"

// Config is the fully-resolved service configuration.
type Config struct {
	HTTPAddr             string        // AUDIT_HTTP_ADDR
	DatabaseURL          string        // AUDIT_DATABASE_URL (postgres DSN)
	RedisURL             string        // AUDIT_REDIS_URL
	InternalSharedSecret string        // AUDIT_INTERNAL_SHARED_SECRET — X-Internal-Token gate on intake; "" => trusted network (ADR-009)
	JWTPublicKeyPEM      string        // AUDIT_JWT_PUBLIC_KEY_PEM — identity RS256 public key; "" => gateway-trust (no in-service verify)
	JWTIssuer            string        // AUDIT_JWT_ISSUER — verified when non-empty
	JWTAudience          string        // AUDIT_JWT_AUDIENCE — verified when non-empty
	ReaderRoles          []string      // AUDIT_READER_ROLES — roles allowed to read the log; empty => any valid token
	IdempotencyTTL       time.Duration // AUDIT_IDEMPOTENCY_TTL_SECONDS
	IntakeEnabled        bool          // AUDIT_INTAKE_ENABLED — toggle the NATS audit.intake consumer (work15 seam)
	LogLevel             string        // AUDIT_LOG_LEVEL (debug|info|warn|error)
}

// Load reads configuration from the environment, applying defaults that make a local
// `go run` work against the docker-compose network names. Required-for-production values
// (DatabaseURL, RedisURL) default to localhost DSNs; main validates connectivity at boot.
func Load() (Config, error) {
	ttl, err := envSeconds("AUDIT_IDEMPOTENCY_TTL_SECONDS", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	intakeEnabled, err := envBool("AUDIT_INTAKE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:             env("AUDIT_HTTP_ADDR", ":8094"),
		DatabaseURL:          env("AUDIT_DATABASE_URL", "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"),
		RedisURL:             env("AUDIT_REDIS_URL", "redis://localhost:6379/0"),
		InternalSharedSecret: env("AUDIT_INTERNAL_SHARED_SECRET", ""),
		JWTPublicKeyPEM:      env("AUDIT_JWT_PUBLIC_KEY_PEM", ""),
		JWTIssuer:            env("AUDIT_JWT_ISSUER", ""),
		JWTAudience:          env("AUDIT_JWT_AUDIENCE", ""),
		ReaderRoles:          envCSV("AUDIT_READER_ROLES", []string{"admin", "compliance"}),
		IdempotencyTTL:       ttl,
		IntakeEnabled:        intakeEnabled,
		LogLevel:             env("AUDIT_LOG_LEVEL", "info"),
	}, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// envCSV parses a comma-separated list. A set-but-empty value yields an empty slice
// (an explicit "no roles required"); an unset value yields def.
func envCSV(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envSeconds(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer (seconds), got %q", key, v)
	}
	return time.Duration(n) * time.Second, nil
}

func envBool(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s must be a boolean, got %q", key, v)
	}
	return b, nil
}
