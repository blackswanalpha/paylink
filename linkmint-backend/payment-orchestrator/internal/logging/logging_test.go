package logging

import (
	"log/slog"
	"testing"
)

func TestNewAndLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "warning", "error", "bogus", ""} {
		if New(lvl, "svc") == nil {
			t.Fatalf("New(%q) returned nil", lvl)
		}
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"junk":  slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
