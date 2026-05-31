# payment-orchestrator (work02)

The **conductor** of the PayLink payment lifecycle. It owns the *flow* (paylink-service owns the
*record*): it initiates a payment for a PayLink, consumes lVM chain events to advance lifecycle
state, reconciles against on-chain truth, and exposes a coherent lifecycle to clients.

Stack: **Go / chi** (ADR-003). This is the **reference Go/chi service template** for the LinkMint
app layer — work03 (proof-validator), work13 (audit-log), work20 (escrow), work23 (settlement),
work24 (wallet), work27 (reconciliation) copy this layout.

## Invariants

- **A.1 Non-custodial** — stores orchestration state only; never funds or fund-moving credentials.
- **A.3 Settlement truth from chain** — settlement comes from on-chain quorum. The orchestrator is
  a *projection + driver*, never an authority: it reads `paylink_getPayLink` and reacts to chain
  events; it never invents settlement.
- **A.7 Anti-replay** — duplicate events/callbacks never double-advance (FSM terminal guard +
  `applied_chain_events` dedupe + `Idempotency-Key`).
- **A.4 Rail-agnostic** — `rail` is an opaque routing label; no rail-specific fields cross the API.

## Lifecycle (a strict projection of the on-chain PayLink FSM)

| on-chain PayLink status | payment lifecycle state |
|-------------------------|-------------------------|
| `CREATED`               | `AWAITING_PAYMENT`      |
| `VERIFIED`              | `SETTLED`               |
| `CANCELLED`             | `CANCELLED`             |
| `FAILED`                | `FAILED`                |

Edges mirror the chain machine out of `CREATED`; the three settlement states are terminal. See
`internal/lifecycle`.

## API (`/v1`)

- `POST /v1/payments` — initiate a payment for an existing, payable PayLink. Requires an
  `Idempotency-Key` header. Body: `{"paylink_id": "0x…", "rail": "mpesa|card|bank|crypto"}`.
  → `201` `{ id, paylink_id, rail, status, created_at, updated_at }`.
- `GET /v1/payments/{id}` — payment status, reconciled against on-chain truth.
- `GET /internal/healthz`, `GET /internal/readyz`, `GET /metrics`.

Errors use the standard envelope: `{"error":{"code","message","details","trace_id"}}`.

## How lifecycle advances

1. **Event-driven (fast path):** a WebSocket subscriber on the lVM datastream (`/ws`) consumes
   `paylink.verified|cancelled|failed` and applies them idempotently.
2. **Read reconcile (safety net):** `GET` reads `paylink_getPayLink` and advances the record if the
   chain is ahead — closing any gap from a missed event during a reconnect.

Both funnel through one atomic, idempotent store operation (FOR UPDATE lock + FSM projection +
`(paylink_id, seq)` dedupe).

## Layout

```
cmd/payment-orchestrator/   bootstrap, graceful shutdown
internal/
  config/        env-only config
  logging/       slog JSON
  httpx/         error envelope, middleware (correlation id, logging, recover), JSON helpers
  metrics/       Prometheus collectors + middleware
  lifecycle/     payment FSM (projection of the on-chain PayLink FSM)
  domain/        Payment + Service + outbound ports (Store, ChainReader, PayLinkLookup, Publisher)
  store/memory   in-memory Store (tests/dev)
  store/postgres pgx Store + embedded numbered migrations
  chain/         lVM JSON-RPC client + event contract; chain/wsstream is the WS EventSource
  paylinks/      paylink-service HTTP client
  events/        domain-event publisher seam (Kafka/SQS deferred to work15)
  idempotency/   Redis-backed Idempotency-Key store
  server/        chi router + /v1 handlers
  subscriber/    bridges the chain EventSource to the domain service
```

## Build / test / run

```bash
make build                 # go build ./...
make test                  # fast unit tests (no Docker)
make test-integration      # + testcontainers (postgres:16, redis:7) — needs Docker
make cover                 # combined coverage; DoD gate (>=80%). Last run: 88.7%
make lint                  # go vet + gofmt check
make run                   # PAYMENT_* env from .env / environment
```

Coverage note: the postgres store and the Redis adapter are exercised by the **testcontainers
integration tier** (`-tags=integration`); the fast unit tier uses in-memory/fake doubles. The
combined figure (`make cover`) is the coverage gate.

## Seams / deferred (filed in `workload/backlog.md`)

- **Event transport** — `events.LogPublisher` is a seam; real Kafka/SQS is **work15**.
- **Double-entry ledger** — settlement ledger entries are **work16/work23** (settlement-service);
  the orchestrator records lifecycle only, no money movement.
- **Auth** — the api-gateway (**work05**) terminates auth; this service trusts upstream today.
- **Rail callbacks / proof verification** — **work03** (proof-validator) and **work04** (mpesa
  adapter); out of scope here.
