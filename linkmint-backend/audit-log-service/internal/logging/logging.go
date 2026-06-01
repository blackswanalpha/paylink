// Package logging builds the service's structured JSON logger (slog).
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog.Logger at the given level, tagged with the service name.
func New(level, service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(h).With("service", service)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
