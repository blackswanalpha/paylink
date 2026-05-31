# work18 — observability (tracing, metrics, structured logs)

> **Seeded** — expand with `/work 18` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** OpenTelemetry + Prometheus + structlog/slog · **Depends on:** — · **Flow:** [flow18](../flow/flow18.md)
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
- [ ] A request's trace_id propagates through ≥2 services + one event hop.
- [ ] `/metrics` scraped by local Prometheus for all services + the node.
- [ ] Logs are structured JSON with correlation id; no secrets/PII.
- [ ] Passes the Infra/CI checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Full stack": trigger a flow, follow one trace across
services in Tempo/Jaeger, confirm metrics in Prometheus.
