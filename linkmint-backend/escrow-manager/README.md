# escrow-manager (work20)

Conditional PayLink release/refund — Go/chi, port **8098**, owns the **`escrow`** Postgres schema.

Hold the *release decision* for a PayLink until a condition is met (delivery confirmation,
time-lock, or N-of-M approval), then emit a release or refund **instruction**. Strictly
**non-custodial (A.1)**: no balance columns, no wallet, no chain-write client — funds never
touch this service. Funding truth arrives from the chain via the `chain.paylink.verified` bus
event (A.3); `escrow.released` / `escrow.refunded` instruct the settlement layer.

## State machine (`internal/fsm`, engine copied from `paylink-chain/internal/fsm`)

```
WAITING ──ConditionsMet(funded ∧ satisfied)──► CONDITIONS_MET ──Release──► RELEASED
WAITING ──Timeout(timeout_at reached)────────────────────────────────────► REFUNDED
WAITING | CONDITIONS_MET ──Dispute───────────────────────────────────────► DISPUTED
```

- `funded` is a **column flag** set by the bus consumer, not a state.
- `ConditionsMet` + `Release` are applied together in one DB transaction, so `CONDITIONS_MET`
  is never persisted.
- `DISPUTED` is terminal here (resolution = work22) and blocks the sweeper and the consumer.

## Condition types

| type | params | satisfied when | confirm |
|---|---|---|---|
| `delivery_confirmation` | none | creator confirmed | creator only |
| `time_lock` | `release_at` (future, < `timeout_at`) | `release_at` reached | 409 — releases automatically |
| `multi_party_approval` | `approvers[]`, `threshold` (1..len) | distinct approvals ≥ threshold | approvers only (idempotent PK) |

## API (`/v1`, gateway-fronted; errors use the standard envelope)

- `POST /v1/escrows` — create (201). Requires `Idempotency-Key` and the gateway-injected
  `X-Creator-Addr`. Duplicate `pl_id` → 409 `ESCROW_EXISTS`. Publishes `escrow.created`.
- `GET /v1/escrows/{id}` — fetch (404 `ESCROW_NOT_FOUND`).
- `GET /v1/escrows?state=&limit=` — caller's escrows (creator-scoped; limit 20, max 100).
- `POST /v1/escrows/{id}/confirm` — record confirmation/approval; releases (in the same tx)
  when funded + satisfied. 403 `NOT_PARTICIPANT`, 409 `INVALID_STATE`/`CONDITION_NOT_CONFIRMABLE`.
- `POST /v1/escrows/{id}/dispute` — body `{"reason"}`; participants only → `DISPUTED`.
- `/internal/healthz`, `/internal/readyz` (postgres + redis), `/metrics`
  (`escrow_transitions_total{kind}`, `escrow_events_consumed_total{result}`, `escrow_sweeps_total`).

## Events (work15 bus, topic `escrow`; consumed: topic `chain`)

Publishes `escrow.created` / `escrow.released` / `escrow.refunded` / `escrow.disputed` after the
owning transaction commits. Consumes `chain.paylink.verified` (group `escrow-manager`): sets
`funded` + `funded_tx_hash`, deduplicated via a work17 **DbDedupe** row in
`escrow.processed_events` written on the same transaction as the funded-write; a handler error
leaves the offset uncommitted (redelivery).

## Sweeper

Every `ESCROW_SWEEP_INTERVAL_SECONDS` (default 10): (1) release due **funded** time_locks, then
(2) refund timeouts. Both are CAS updates (`UPDATE … WHERE state='WAITING'`); errors are logged
and the loop never dies.

## Configuration (env only — see `.env.example`)

`ESCROW_HTTP_ADDR` (:8098), `ESCROW_DATABASE_URL`, `ESCROW_REDIS_URL`, `ESCROW_LOG_LEVEL`,
`ESCROW_IDEMPOTENCY_TTL_SECONDS` (86400), `ESCROW_EVENT_PUBLISHER_MODE` (log|kafka),
`ESCROW_EVENT_CONSUMER_ENABLED` (true), `KAFKA_BROKERS`, `ESCROW_SWEEP_ENABLED` (true),
`ESCROW_SWEEP_INTERVAL_SECONDS` (10), `ESCROW_DEFAULT_TIMEOUT_SECONDS` (86400),
`ESCROW_DEV_CREATOR_ADDR` (dev-only X-Creator-Addr fallback; empty ⇒ 401 on mutating routes).

## Develop

```sh
make build            # compile
make test             # unit tests (no Docker)
make test-integration # + testcontainers postgres
make cover            # combined coverage (>=80% gate), per-package profiles
make lint             # go vet + gofmt check
make run              # local run (uses env / defaults)
```
