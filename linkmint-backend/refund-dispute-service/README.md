# refund-dispute-service (work22)

Refunds and disputes/chargebacks for LinkMint — spec §2.9, Phase 2 / Beta. Python/FastAPI, owns the
`refund` Postgres schema, port **8100**. Strictly **non-custodial** (rules.md A.1): it records refund
and dispute state and emits **instructions** (rail reversal, clawback) — it never holds or moves
funds.

## What it does

- **Refunds** (sender/merchant-initiated): `REQUESTED → PROCESSING → COMPLETED` (full + partial),
  with `REJECTED` / `FAILED` terminal branches. Eligibility requires the payment to be `SETTLED`
  (payment-orchestrator); the original amount — for the full/partial flag and the cumulative cap — is
  resolved from the `verified_paylinks` projection (fed by `chain.paylink.verified`) with a
  paylink-service fallback.
- **Disputes** (rail-initiated): an HMAC-signed provider webhook opens a dispute; the merchant adds
  evidence within the rail-imposed window and submits; the provider's resolution arrives on the same
  webhook. `OPEN → SUBMITTED → WON|LOST`, plus `EXPIRED` when the evidence window lapses. A lost or
  expired dispute requests a **clawback** from the merchant's next payout.

## API

| Route | Auth | Notes |
|---|---|---|
| `POST /v1/refunds` | JWT (RS256) | `{payment_id, amount_minor, currency?, reason?, org_id?, merchant_id?}` → 201 `REQUESTED`; Idempotency-Key honored |
| `POST /v1/refunds/{id}/approve` | JWT org-admin / platform-admin | → `PROCESSING` (instructs the rail reversal) |
| `POST /v1/refunds/{id}/reject` | JWT org-admin / platform-admin | → `REJECTED` |
| `GET /v1/refunds/{id}` · `GET /v1/refunds?payment_id=` | JWT | org-scoped reads |
| `POST /v1/disputes/webhooks/{provider}` | **HMAC** (`X-Signature`, no JWT) | intake + resolution, dispatched on payload `kind` (`dispute.opened` / `dispute.resolved`); replays are no-ops |
| `POST /v1/disputes/{id}/evidence` | JWT org-member | window-enforced |
| `POST /v1/disputes/{id}/submit` | JWT org-admin | → `SUBMITTED` |
| `GET /v1/disputes/{id}` | JWT | includes evidence + `evidence_due_at` |

All responses use the standard envelope `{"error": {"code", "message", "details", "trace_id"}}`.
RBAC is authorized from the RS256 token claims (no memberships table here, per rules.md).

## Events (catalog.md)

Published to the `refund` topic: `refund.requested`, `refund.approved`, `refund.rejected`,
`refund.reversal.instructed`, `refund.processing`, `refund.completed`, `refund.failed`,
`refund.clawback.requested`. To the `dispute` topic: `dispute.opened`, `dispute.evidence_added`,
`dispute.submitted`, `dispute.won`, `dispute.lost`, `dispute.expired`. All via the transactional
outbox (`refund.refund_events`) drained by the work15 relay. Consumes `chain.paylink.verified` (the
original-amount projection, A.3) with RedisDedupe + durable DbDedupe (exactly-once effect).

## Seams (deliberate MVP boundaries)

- **Rail reversal is instruction-only.** No rail adapter supports reversal yet — the MPesa adapter is
  STK-push-only (`adapters/mpesa/DESIGN.md`: "never B2C/reversal/sweep") and the card/crypto/bank
  adapters are work28–30. `approve` emits `refund.reversal.instructed` (the instruction) and the
  `RailReversalRegistry` returns a deterministic `reversal_ref`. A real per-rail adapter slots into
  the registry later with no call-site change. In dev (`REFUND_REVERSAL_SIMULATE=true`) the sweeper
  advances `PROCESSING → COMPLETED`.
- **Clawback is a published contract for settlement (work23).** work23 isn't built yet, so a lost/
  expired dispute writes a `refund.clawback.requested` outbox row that settlement will consume —
  there is no synchronous coupling. (Same pattern as work11's audit sink before work13.)
- **Ledger posting (A.6) is OFF by default** (`REFUND_LEDGER_POSTING_ENABLED=false`); settlement
  (work23) is the canonical ledger writer (work16 deferred per-service posting).
- **Escrow-dispute resolution is out of scope.** escrow-manager (work20) leaves a `DISPUTED` seam;
  resolving escrow disputes is a follow-up — work22 covers payment refunds + rail chargebacks.

## Configuration

12-factor; all `REFUND_*` env vars (see `.env.example`). Notable: `REFUND_AMOUNT_VALIDATION`
(`strict` rejects when the original amount is unresolvable; `lenient` accepts the caller amount, with
merchant approval as the gate — the dev compose runs `lenient`, prod runs `strict`),
`REFUND_WEBHOOK_SECRETS` (`<provider>:secret;…`), `REFUND_CLAWBACK_MODE` (`event`|`noop`).

## Develop

```sh
python3 -m venv .venv && . .venv/bin/activate
pip install -e ".[dev]" ../idempotency-python ../telemetry-python ../eventbus-python
ruff check . && black --check . && mypy . && pytest      # unit suite, ≥80% coverage gate
```

Integration tests (real Postgres/Redis via testcontainers) run under `pytest` when Docker is
available and skip otherwise. In the stack: `docker compose up -d refund-dispute-service` (migrations
run on start; readiness at `/internal/readyz`).
