# Standard — how code is implemented

This expands the terse "Code Style" section of [`../CLAUDE.md`](../CLAUDE.md) into an
actionable standard. It does not contradict it. When in doubt, match the existing code
in `paylink-chain/`.

---

## Go — `paylink-chain/` (the lVM node)

**Formatting & hygiene**
- `gofmt` clean, `go vet ./...` clean (`make fmt`, `make lint`).
- Standard Go project layout; packages under `internal/` are not importable externally.
- Keep `go.mod` tidy (`make tidy` / `go mod tidy`).

**Adding a new transaction type** (the canonical chain extension, from `CLAUDE.md`):
1. Add the constant in `internal/types/transaction.go`.
2. Add the payload struct (same file or a focused file in `types/`).
3. Add a `case` in the executor switch in `internal/chain/executor.go`.
4. Emit event kinds in `internal/events/event.go`.
5. Write tests — extend the table-driven tests in `internal/chain/executor_test.go`
   and add integration coverage under `test/`.

**Testing**
- Table-driven tests; deterministic expectations for state roots and merkle roots
  (mirror the patterns in `internal/chain/executor_test.go`).
- Run `go test ./... -count=1` (no cache) before declaring done.
- Integration tests live in `paylink-chain/test/` — there are already 64; new chain
  behavior gets integration coverage there too.

**Determinism**
- Block production and state transitions must be deterministic. No wall-clock or
  randomness inside execution paths that affect state (committee selection uses ECVRF,
  not `math/rand`).

---

## Backend services — stack per service (ADR-003)

Per `backendfeatures.md` (ADR-003 in [decisions.md](decisions.md)), backend services are
**Python/FastAPI** by default, with **Go/chi** for the performance/throughput-sensitive ones.
**TypeScript is only for the JS SDK and the web app.** The lVM is Go (see the Go section above).

| Stack | Services |
|-------|----------|
| **Go/chi** | payment-orchestrator, proof-validator, escrow-manager, settlement-service, wallet-service, audit-log-service, reconciliation-service, adapters framework (mpesa/card/crypto/bank) |
| **Python/FastAPI** | identity-service, merchant-onboarding, admin-backoffice, paylink-service, invoice-subscription, fee-pricing-service, refund-dispute-service, compliance-risk, fraud-detection, notification-service, reporting-analytics |
| **Kong** | api-gateway (DB-less declarative — see **ADR-008**, which amends this row) |
| **TypeScript** | `sdks/javascript`, `apps/web` (clients only) |

Each work item states its stack. When unsure, match the table.

### Service shape — all backend services (12-factor, language-agnostic)
- All config from environment variables — no hard-coded hosts, ports, secrets.
- Structured JSON logs with a correlation ID (`X-Request-Id` / `trace_id`) per request.
- Health (`/internal/healthz`) and readiness (`/internal/readyz`) endpoints; graceful shutdown.
- `/metrics` (Prometheus) endpoint.
- **Observability:** call `telemetry.Init` (Go) / `init_telemetry` (Python) at startup and add the
  shared telemetry middleware first — the libs `telemetry-go` / `telemetry-python` give OpenTelemetry
  tracing (OTLP→Tempo), W3C `traceparent` propagation across HTTP + the Kafka bus, and the standard
  `http_requests_total` / `bus_messages_consumed_total` / `chain_txs_submitted_total` counters
  (**ADR-013**). Tracing is a no-op until `OTEL_EXPORTER_OTLP_ENDPOINT` is set. NO secrets/PII in logs,
  spans, or metric labels (route templates only, never raw paths).
- **API:** RESTful, versioned `/v1/...`. Standard error envelope on every error:
  ```json
  { "error": { "code": "PAYLINK_EXPIRED", "message": "human readable", "details": {}, "trace_id": "..." } }
  ```
  Document endpoints in `docs/api/` (OpenAPI); update SDK clients in the same change.
- **Idempotency:** state-mutating endpoints accept an `Idempotency-Key` header (Redis-backed,
  24h TTL); see the idempotency framework work item.
- **Events:** publish/consume domain events by their logical name over the Kafka transport
  (Redpanda; **ADR-011**, refining ADR-004). The logical name → topic model and producer/consumer
  map is [catalog.md](catalog.md); use the shared client libs `eventbus-go` / `eventbus-python`
  (byte-identical envelope). Producers with a durable outbox drain it via a relay; consumers are
  idempotent (at-least-once).
- **Persistence:** PostgreSQL primary (one schema per service), Redis cache, Kafka/SQS async.
  **All schema changes via numbered migrations** — never edit an applied migration; add a new
  one. Never modify production data directly. No cross-schema foreign keys (opaque id refs).

### Python/FastAPI services
- Python 3.12+, **FastAPI** + Pydantic models for request/response validation.
- Type hints everywhere; `mypy` clean; `ruff` (lint) + `black` (format) clean.
- Config via Pydantic `BaseSettings` from env. `structlog` for JSON logging.
- DB via SQLAlchemy + Alembic numbered migrations (or the project's chosen equivalent).
- Tests with `pytest`; integration tests with testcontainers. **≥80% coverage.**

### Go/chi services (app layer, not the lVM)
- Go, `chi` router. `gofmt` + `go vet` clean.
- Config from env; `slog` JSON logging; graceful shutdown.
- DB via the project's standard Go Postgres layer + numbered SQL migrations.
- The proof-validator and adapters import `paylink-chain/internal/types` + `internal/crypto`
  for **byte-exact** chain wire format and signing — never re-derive these.
- Table-driven tests; integration tests. **≥80% coverage.**

### TypeScript (SDK + web app only)
- `strict` mode. **No `any`.** ESLint + Prettier clean (`npm run lint`).
- The web app calls the API only through the JS SDK (not raw fetch).
- Tests with Jest or Vitest; **≥80% coverage** for the SDK.

---

## Cross-cutting

**Rail-agnostic proof** — every adapter outputs exactly this shape and nothing rail-specific
crosses the boundary:
```json
{ "pl_id": "...", "rail": "mpesa|card|bank|crypto", "tx_id": "...",
  "amount": "...", "timestamp": "...", "sender": "...", "receiver": "...",
  "proof_signature": "..." }
```

**Commits** — Conventional Commits with scope: `feat(paylink-service): add expiry job`,
`fix(consensus): correct quorum threshold`, `docs(workload): seed backlog`.
Prefixes: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`.

**Secrets** — env vars or KMS/Key Vault references only. Never in code, config, or logs.

**Reuse over rewrite** — before adding a primitive (hashing, signing, state access, event
emission), check `paylink-chain/internal/` for an existing one.

**Definition of done** — every change closes against [`definition-of-done.md`](definition-of-done.md)
and is verified per [`verification.md`](verification.md).
