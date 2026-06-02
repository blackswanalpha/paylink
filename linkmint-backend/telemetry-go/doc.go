// Package telemetry is LinkMint's shared OpenTelemetry helper for Go services (work18). It is the
// Go half of the telemetry-go / telemetry-python pair: both wire the same distributed-tracing
// contract (W3C trace context over OTLP), so a trace_id started in one service follows the request
// through HTTP calls and the Kafka event bus into the next — in either language.
//
// Init configures the global TracerProvider (OTLP gRPC export to Tempo) and the W3C propagator.
// Middleware starts a server span per request and makes the active 32-hex trace id the request's
// correlation id (it seeds X-Request-Id), so the service's existing slog `trace_id` field, the
// error envelope, the response header and the trace in Tempo all share ONE id. WrapTransport injects
// that trace context into outbound HTTP calls.
//
// Tracing is OFF (a cheap no-op) unless OTEL_EXPORTER_OTLP_ENDPOINT is set and OTEL_SDK_DISABLED is
// not truthy — so importing telemetry never changes a service's behavior until a collector is wired
// in. The propagator is always installed, costing nothing when no span is recording.
//
// Invariant: spans and the unified correlation id carry only low-cardinality, PII-free data — the
// route TEMPLATE (never a raw path with ids in it), the HTTP method and the status code. Callers must
// not attach request bodies, query strings, auth headers, or rail identifiers to spans.
package telemetry
