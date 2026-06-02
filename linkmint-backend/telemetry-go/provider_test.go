package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestInitDisabled(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:14317")
	sh, err := Init(context.Background(), "svc", "v0")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := sh(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInitNoEndpoint(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	sh, err := Init(context.Background(), "svc", "v0")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := sh(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInitWithEndpoint(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:14317")
	t.Setenv("OTEL_SERVICE_NAME", "override-svc")
	sh, err := Init(context.Background(), "svc", "v0")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := sh(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestHelpers(t *testing.T) {
	if !truthy("YES") || truthy("nope") || truthy("") {
		t.Fatal("truthy")
	}
	if envOr("definitely_missing_xyz", "def") != "def" {
		t.Fatal("envOr default")
	}
	t.Setenv("PRESENT_XYZ", "val")
	if envOr("PRESENT_XYZ", "def") != "val" {
		t.Fatal("envOr present")
	}
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
	if samplerRatio() != 0.25 {
		t.Fatal("ratio")
	}
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "bad")
	if samplerRatio() != 1.0 {
		t.Fatal("ratio default")
	}
	for _, ep := range []string{"https://collector:4317/", "http://x:4317", "collector:4317"} {
		if len(grpcOpts(ep)) == 0 {
			t.Fatalf("grpcOpts(%q) empty", ep)
		}
	}
}
