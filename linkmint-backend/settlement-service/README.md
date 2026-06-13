# settlement-service (work23)

Off-chain settlement lifecycle for LinkMint (Go/chi). Aggregates verified PayLinks into
per-merchant settlements (gross/fee/net), schedules **T+1 payouts**, ingests **rail settlement
files**, and records every monetary flow as a balanced double-entry ledger posting (work16, **A.6**).

**Non-custodial (A.1):** there are no balance columns and no fund movement. A payout is an
*instruction* over the merchant's external rail; LinkMint only records the flow and emits the event.

Owns the `settlement` schema; runs the shared `ledger` migrations. Port **8101**. Spec:
`backendfeatures.md §2.12`.

## API

Merchant-scoped (the gateway authenticates and injects `X-Creator-Addr` = the caller's on-chain
address = the merchant settlement key):

| Method | Path | Purpose |
|---|---|---|
| GET  | `/v1/settlements?status=&limit=` | list the caller's settlements |
| GET  | `/v1/settlements/{id}` | one settlement + its items |
| GET  | `/v1/payouts?status=&limit=` | list the caller's payouts |
| GET  | `/v1/payouts/{id}` | one payout |
| POST | `/v1/payouts` | instruct a payout for a CLOSED settlement (`Idempotency-Key` required) |

Internal / trusted-network only (NOT routed through the gateway; guarded by `SETTLEMENT_INGEST_TOKEN`):

| Method | Path | Purpose |
|---|---|---|
| POST | `/settlements/files/ingest` | ingest a rail settlement file (JSON or CSV), match lines to payouts |

Plus `/internal/healthz`, `/internal/readyz`, `/metrics`. All errors use the standard envelope.

## Events

- **Consumes** (topics `chain`, `merchant`, `refund`): `chain.paylink.verified` (gross + payee),
  `chain.fee.collected` (chain fee, A.5), `merchant.onboarded` / `merchant.bank_account.verified`
  (routing projections), `refund.clawback.requested` (negative offset).
- **Publishes** (topic `settlement`): `settlement.batch_created`, `settlement.completed`,
  `payout.scheduled`, `payout.instructed`, `payout.completed`.

Consumers are idempotent (DbDedupe on `settlement.processed_events`, work17) and poison-safe.

## Build / test / run

```sh
make build          # go build ./...
make lint           # go vet + gofmt -l
make test           # unit tests (no Docker)
make cover          # unit + integration (testcontainers postgres); DoD gate >=80%
make run            # go run ./cmd/settlement-service  (reads SETTLEMENT_* env; see .env.example)
```

Config is 12-factor (env only) — see `.env.example`. Multi-currency, instant payouts, and payout
splitting are Phase 3 (out of scope here). See `DESIGN.md` for the architecture and known seams.
