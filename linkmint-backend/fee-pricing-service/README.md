# fee-pricing-service (work21)

The single source of **"what does this payment cost"**: per-merchant **tier** pricing, per-**rail**
fee schedules, **FX** quoting (rates locked at quote time for audit), and monthly **platform-fee
invoicing**. The service is non-custodial (A.1) — it stores pricing metadata only and never holds or
moves funds. The platform pricing fee here is **distinct** from the on-chain 0.5% PLN inflation fee
(`rules.md` A.5), which is chain-side only.

> Phase 2 / Beta. Spec: `backendfeatures.md` §2.8. Depends on work10 (merchant-onboarding) for
> merchant tiers. Volume-tier auto-upgrade + FX hedging are deferred to Phase 3.

## API

| Method | Path | Purpose | Authz |
|---|---|---|---|
| `POST` | `/v1/pricing/quote` | Quote `{gross, platform_fee, rail_fee, net, breakdown}` per (tier, rail). | JWT (any caller) |
| `GET` | `/v1/fx/rates` | Cached FX rates (`?base=&quote=`). | JWT |
| `GET` | `/v1/pricing/tiers` | List fee tiers (`?active=`). | JWT + platform admin |
| `GET` | `/v1/pricing/merchants/{id}` · `/v1/merchants/{id}/pricing` | A merchant's current pricing config. | JWT (org-member or admin) |
| `POST` | `/v1/internal/accruals` | Record a realized platform fee (idempotent on `source_ref`). | X-Internal-Token |
| `POST` | `/v1/internal/invoices/platform-fee/run` | Generate monthly platform-fee invoices for a period. | X-Internal-Token |

The gateway exposes `/v1/pricing` and `/v1/fx` (pass-through, RS256 self-verify). The spec path
`/v1/merchants/{id}/pricing` is served in-service for east-west callers but namespaced under
`/v1/pricing/merchants/{id}` at the edge to avoid colliding with merchant-onboarding's `/v1/merchants`.
`/v1/internal/*` is trusted-network only (never routed at the gateway).

State-mutating routes honour `Idempotency-Key`. Errors use the standard envelope
`{"error": {"code", "message", "details", "trace_id"}}`. Health: `/internal/healthz`,
`/internal/readyz`; metrics at `/metrics`.

## Events

- **Publishes** (outbox relay): `pricing.fee_quote.issued`, `fx.rate.updated`,
  `invoice.platform_fee.issued`. Per the bus's first-dot-segment topic rule these route to the
  `pricing`/`fx`/`invoice` topics (created by `redpanda-init`); see the `catalog.md` footnote.
- **Consumes** (topic `merchant`): `merchant.onboarded` (seed default-tier pricing + capture org_id),
  `merchant.fee_tier.changed` (update the tier). Unknown tiers fall back to `standard` with a warning.

## Money

Integer **minor units** in `NUMERIC(38,0)`; FX rates `NUMERIC(38,18)`. No floats. Per (tier, rail):
`platform_fee = round(gross × tier.pct_bps / 10_000) + tier.fixed`,
`rail_fee = round(gross × rail.pct_bps / 10_000) + rail.fixed`, `net = gross − platform_fee − rail_fee`.
Cross-currency quotes multiply gross by the **locked** FX rate (stored on the `quotes` row for audit).

## Config (`PRICING_*`)

`HTTP_PORT` (8097), `DATABASE_URL`, `REDIS_URL`, `JWT_PUBLIC_KEY_PEM`/`JWT_ISSUER`/`JWT_AUDIENCE`,
`ADMIN_USER_ROLES`, `EVENT_PUBLISHER_MODE` (`log|noop|kafka`) + `KAFKA_BROKERS`,
`EVENT_CONSUMER_ENABLED`, `FX_PROVIDER` (`static|http`) + `FX_STATIC_RATES`/`FX_FALLBACK_RATES`/
`FX_CACHE_TTL_SECONDS`, `INVOICE_SWEEP_ENABLED`/`INVOICE_SWEEP_INTERVAL_SECONDS`,
`INTERNAL_SHARED_SECRET`, `ACCRUAL_FROM_EVENTS`, `LEDGER_POSTING_ENABLED`, `DEFAULT_CURRENCY`. See
`.env.example`.

## Develop

```bash
pip install -e ".[dev]"
pip install ../idempotency-python ../telemetry-python   # shared libs the app imports eagerly
ruff check . && black --check . && mypy . && pytest
```

Build/run via the repo `docker-compose.yml` (`fee-pricing-service`, port 8097). The image runs
`alembic upgrade head` then `uvicorn` (see `entrypoint.sh`).
