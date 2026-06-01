# notification-service (work14)

Multi-channel delivery of LinkMint domain events to users and merchants — **SMS + email** in
Phase 1 — with template rendering, a durable delivery log, and **Celery/Redis retry** with
exponential backoff. Spec: `backendfeatures.md` **§2.7** (the `§2.18` label in `workload/work/work14.md`
is a stale reference — §2.18 is admin-backoffice).

**Non-custodial (invariant A.1):** this service only sends messages and writes a delivery log. It
moves, holds, escrows, or sweeps **no funds** and signs no chain transaction.

## What it does (Phase 1)

```
domain event ──▶ NotificationEventConsumer.handle(name, payload)   # the work15 bus chokepoint
              ──▶ resolve recipient (RecipientResolver)
              ──▶ fan out to channels with an active template (TemplateRegistry)
              ──▶ render (string.Template placeholders)
              ──▶ create notify.deliveries rows (QUEUED)  +  enqueue Celery deliver(delivery_id)
worker        ──▶ DeliveryRunner.run_once: provider.send → SENT,  or FAILED→retry(countdown)→…→EXHAUSTED
```

- **Channels:** `console` sandbox (default; the verified Phase-1 path) + config-gated `http`
  drop-ins — Africa's Talking (SMS), SendGrid (email). Provider secrets are env-only (`SecretStr`).
- **Retry backoff:** `30s, 2m, 10m, 1h, 6h` (max 5 retries). `notify.deliveries` is the durable
  system-of-record (status / attempts / last_error / next_retry_at); Celery `apply_async(countdown=)`
  drives scheduling. `attempts` increments on failure only.
- **Events supported:** `paylink.verified`, `payment.failed` (SMS + email, `en`).

## Event bus (work15) seam

The real Kafka/SQS transport is **work15** (`Depends on: 15`, still `todo`). Until it lands, the
typed `NotificationEventConsumer.handle(name, payload)` chokepoint is driven by the trusted-network
HTTP intake `POST /v1/notifications` (the bus stand-in) and by tests directly — the future bus
subscriber calls the same method. This mirrors how paylink/compliance shipped ahead of work15.

## API

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/v1/notifications` | trusted-network (`X-Internal-Token`, ADR-009) | intake an event; honors `Idempotency-Key`; → `201 {delivery_ids}` |
| GET | `/internal/deliveries/{id}` | trusted-network | delivery status (recipient **masked**) — not the Phase-2 public delivery-log API |
| GET | `/internal/healthz`, `/internal/readyz`, `/metrics` | — | health / readiness (db + redis + broker soft-dep) / Prometheus |

`POST /v1/notifications` body: `{event_kind, user_id, locale?, data{str:str}, contact?{phone?,email?}}`.

## PII minimization (invariant)

Other services emit events carrying **ids/metadata only, never raw PII**. The destination contact
reaches this service via the `RecipientResolver` seam: the Phase-1 `inline` resolver reads `contact`
from the *trusted intake call* (the caller already holds it); the deferred `identity` resolver
(config flip) fetches it from identity-service so even the intake carries no PII. Phone/email are
**masked in every log line and in the GET response**; the `notify.deliveries.recipient` column
legitimately stores the real value (per §2.7).

## Out of scope (Phase 2+)

`/v1/webhooks` CRUD + HMAC-signed webhook delivery + the public delivery-log API; push (FCM);
per-merchant rate limits; circuit-breaker. The `notify.webhooks` table is created as forward-schema.

## Develop / test / run

```bash
pip install -e ".[dev]"
ruff check . && black --check . && mypy .
pytest                                   # unit + integration (testcontainers); ≥80% gate

# Local (needs Postgres + Redis):
alembic upgrade head
uvicorn app.main:app --reload --port 8095
celery -A app.celeryapp.app:celery_app worker -Q notify --loglevel INFO
```

## Config (NOTIFY_* env)

`NOTIFY_DATABASE_URL`, `NOTIFY_REDIS_URL` (idempotency, db `/0`), `NOTIFY_CELERY_BROKER_URL`
(broker, db `/1`), `NOTIFY_SMS_PROVIDER` (`console`|`http`), `NOTIFY_EMAIL_PROVIDER`,
`NOTIFY_RECIPIENT_RESOLVER` (`inline`|`identity`), `NOTIFY_INTERNAL_SHARED_SECRET`,
`NOTIFY_AFRICASTALKING_*`, `NOTIFY_SENDGRID_API_KEY`, `NOTIFY_CONSOLE_FAIL_RECIPIENTS` (retry-test
hook). All config is env-sourced; secrets are `SecretStr` and never logged.
