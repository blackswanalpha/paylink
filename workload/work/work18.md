# work18 — observability (tracing, metrics, structured logs)

> **Done 2026-06-02.** Shipped the paired shared libs `telemetry-go` (97% cov) + `telemetry-python`
> (98% cov) — thin OTel middleware (no contrib/instrumentation deps), OTLP→Tempo, env-gated no-op by
> default. Trace context rides W3C `traceparent` in **Kafka record headers** (instrumented inside
> eventbus-go/python; the byte-identical Envelope is untouched). The telemetry HTTP/ASGI middleware
> runs outermost and seeds `X-Request-Id` with the OTel trace id, so logs/envelope/response/Tempo share
> one id. Wired into all 5 Go + 6 Python services; added `chain_txs_submitted_total` (proof-validator)
> + Python `http_requests_total` + `bus_messages_consumed_total`. Local stack = Prometheus + Tempo +
> Grafana behind a docker-compose `observability` profile + `monitoring/` configs; node runs
> `--metrics`. CI gained telemetry-go/python jobs + per-service installs. ADR-013; invariants PASS.

- **Status:** done · **Owner:** service-builder · **Stack:** OpenTelemetry + Prometheus + structlog/slog · **Depends on:** — · **Flow:** [flow18](../flow/flow18.md)
- **Phase:** 1 / MVP (cross-cutting) · **Spec:** backendfeatures.md §"Observability"

## Goal
Uniform observability across all services: OpenTelemetry tracing (OTLP), Prometheus `/metrics`,
and structured JSON logging with correlation ids — so the end-to-end flow is debuggable.

## In scope
- Shared tracing/logging init for Python (structlog + OTel) and Go (slog + OTel).
- `X-Request-Id` propagation → `trace_id` field across HTTP + event bus boundaries.
- Standard metrics (`http_requests_total`, `*_messages_consumed_total`, `chain_txs_submitted_total`)
  + a local Prometheus + tracing backend (Tempo/Jaeger) in docker-compose.
- The lVM already exposes Prometheus metrics (`paylinkd --metrics`) — scrape it too.

## Out of scope
- SLO-keyed production alerting + Grafana dashboards depth (Phase 2/3).
- Log aggregation stack (ELK/Loki) production deployment (Phase 3).

## Invariants that apply
- No secrets/PII in logs, traces, or metric labels; non-custodial.

## Reuse first
- The lVM metrics in `paylink-chain/internal/metrics`; the OTel deps already in the chain's go.mod.

## Acceptance criteria
- [x] A request's trace_id propagates through ≥2 services + one event hop. (W3C propagation
  unit-proven in telemetry-go's HTTP round-trip + eventbus-go/python header round-trip tests; live path:
  paylink-service→compliance-risk HTTP hop + payment-orchestrator→notification-service event hop.)
- [x] `/metrics` scraped by local Prometheus for all services + the node. (`monitoring/prometheus/
  prometheus.yml` scrapes every service + `paylink-chain:9090` + Kong `:8100`.)
- [x] Logs are structured JSON with correlation id; no secrets/PII. (Existing structlog/slog; the
  invariant audit confirmed route-templates-only labels, no PII in spans/labels.)
- [x] Passes the Infra/CI checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Full stack": trigger a flow, follow one trace across
services in Tempo/Jaeger, confirm metrics in Prometheus.

## Notes / log
- 2026-06-12 — audit found a scrape gap: invoice-subscription (8096) and fee-pricing-service (8097)
  landed after work18 with telemetry wired but were missing from `monitoring/prometheus/prometheus.yml`,
  so their metrics were silently dropped. Targets added (plus escrow-manager:8098 with work20). Suites
  fresh-green: telemetry-go 97.4%, telemetry-python 98.1%.
