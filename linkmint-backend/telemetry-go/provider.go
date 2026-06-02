package telemetry

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ShutdownFunc flushes and stops the telemetry pipeline. It is always safe to call — when tracing is
// off it is a no-op — so callers can unconditionally `defer shutdown(ctx)`.
type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

// Init installs the global W3C propagator and, when an OTLP endpoint is configured, a batched
// TracerProvider that exports spans over gRPC. It returns a shutdown function and an error.
//
// Honored environment (the standard OTel vars, so ops tooling works unchanged):
//
//	OTEL_SDK_DISABLED            truthy → force a no-op (propagator still installed)
//	OTEL_EXPORTER_OTLP_ENDPOINT  OTLP gRPC collector, e.g. http://tempo:4317; empty → no-op export
//	OTEL_SERVICE_NAME            overrides the serviceName argument
//	OTEL_TRACES_SAMPLER_ARG      parent-based ratio in [0,1] (default 1.0 — sample all, for local dev)
//	DEPLOY_ENV                   deployment.environment resource attr (default "local")
//
// The propagator is installed even when export is off, so a request that arrives carrying a
// traceparent still threads one trace_id through this service's logs; flipping on the endpoint later
// lights up export with no code change.
func Init(ctx context.Context, serviceName, version string) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if truthy(os.Getenv("OTEL_SDK_DISABLED")) {
		return noopShutdown, nil
	}
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		return noopShutdown, nil
	}
	if n := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); n != "" {
		serviceName = n
	}

	exp, err := otlptracegrpc.New(ctx, grpcOpts(endpoint)...)
	if err != nil {
		return noopShutdown, err
	}
	res := resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", version),
		attribute.String("deployment.environment", envOr("DEPLOY_ENV", "local")),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplerRatio()))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// grpcOpts derives the OTLP gRPC dial options from an endpoint URL, inferring TLS from the scheme:
// http:// → insecure (the local Tempo default), https:// → TLS, bare host:port → insecure.
func grpcOpts(endpoint string) []otlptracegrpc.Option {
	insecure := true
	ep := endpoint
	switch {
	case strings.HasPrefix(ep, "http://"):
		ep, insecure = strings.TrimPrefix(ep, "http://"), true
	case strings.HasPrefix(ep, "https://"):
		ep, insecure = strings.TrimPrefix(ep, "https://"), false
	}
	ep = strings.TrimSuffix(ep, "/")
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(ep)}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	return opts
}

func samplerRatio() float64 {
	if v := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
	}
	return 1.0
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
