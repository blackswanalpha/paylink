# invoice-subscription (work19)

Multi-line **invoices** that aggregate into a single backing **PayLink**, with the
`DRAFT → OPEN → PAID | VOID | OVERDUE` lifecycle. Settlement truth comes from the chain — an invoice
becomes `PAID` only when the `chain.paylink.verified` domain event arrives for its PayLink. The
service never holds funds (non-custodial) and stores no rail-specific data (rail-agnostic).

> Phase 2 / Beta. Recurring **subscriptions** (and dunning/proration) are deferred to **work31**
> (Phase 3). Spec: `backendfeatures.md` §2.19.

## API (`/v1/invoices`, JWT merchant)

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/invoices` | Create a DRAFT invoice with lines (aggregated totals). |
| `GET` | `/v1/invoices` | List the merchant's invoices (`?status=&limit=&offset=`). |
| `GET` | `/v1/invoices/{id}` | Read an invoice + its lines (reflects OVERDUE lazily). |
| `POST` | `/v1/invoices/{id}/finalize` | Mint the backing PayLink, move DRAFT → OPEN (one-way). |
| `POST` | `/v1/invoices/{id}/void` | Void the invoice (blocked once PAID → `409 ALREADY_PAID`). |

State-mutating routes honour `Idempotency-Key`. Errors use the standard envelope
`{"error": {"code", "message", "details", "trace_id"}}`. Health: `/internal/healthz`,
`/internal/readyz`; metrics at `/metrics`.

## Events

- **Publishes** (outbox → topic `invoice`): `invoice.created`, `invoice.finalized`, `invoice.paid`,
  `invoice.overdue` (+ `invoice.voided`).
- **Consumes** (topic `chain`): `chain.paylink.verified` → marks the backed invoice PAID.

## Money

Integer **minor units** in `NUMERIC(38,0)`. `line.total = round(quantity × unit_price)`;
`invoice.tax = Σ round(line.total × tax_rate)`; `total = subtotal + tax`. The PayLink `amount` is
`int(total)`.

## Config (`INVOICE_*`)

`HTTP_PORT` (8096), `DATABASE_URL`, `REDIS_URL`, `JWT_PUBLIC_KEY_PEM`/`JWT_ISSUER`/`JWT_AUDIENCE`,
`PAYLINK_SERVICE_URL`/`PAYLINK_INTERNAL_TOKEN`, `EVENT_PUBLISHER_MODE` (`log|noop|kafka`) +
`KAFKA_BROKERS`, `EVENT_CONSUMER_ENABLED`, `OVERDUE_SWEEP_ENABLED`/`OVERDUE_SWEEP_INTERVAL_SECONDS`,
`DEFAULT_CURRENCY`.

## Develop

```bash
pip install -e ".[dev]"
pip install ../idempotency-python ../telemetry-python   # shared libs the app imports eagerly
ruff check . && black --check . && mypy . && pytest
```

Build/run via the repo `docker-compose.yml` (`invoice-subscription` service, port 8096). The image
runs `alembic upgrade head` then `uvicorn` (see `entrypoint.sh`).
