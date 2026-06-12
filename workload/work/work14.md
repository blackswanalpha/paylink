# work14 — notification-service (SMS/email; webhooks in Phase 2)

> **Seeded** — expand with `/work 14` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI (+ Celery/Redis) · **Depends on:** 15 · **Flow:** [flow14](../flow/flow14.md)
- **Phase:** 1 / MVP (SMS/email) · **Spec:** backendfeatures.md §2.18

## Goal
Multi-channel delivery with retry semantics and templates. Phase 1: SMS (Africa's Talking /
Twilio) + email (SendGrid/SES). Push (FCM) and HMAC-signed webhooks are Phase 2.

## In scope (Phase 1)
- Consume domain events; render templates; deliver via SMS + email.
- Retry with exponential backoff (30s,2m,10m,1h,6h; max 5); delivery log.
- Owns `notify` schema (webhooks, deliveries, templates); template placeholders.

## Out of scope (Phase 2+)
- `/v1/webhooks` registration + HMAC-signed webhook delivery + delivery-log API (Phase 2).
- Push (FCM); per-merchant rate limits + webhook UI (Phase 3).

## Invariants that apply
- Non-custodial; provider secrets via env/KMS; PII minimization in payloads.

## Reuse first
- work01 Python/FastAPI layout; event bus (work15) as the trigger source; standard error envelope.

## Acceptance criteria
- [x] Domain event → templated SMS + email delivered (sandbox providers); retries on failure.
- [x] Delivery log persisted; events consumed per a template registry.
- [x] Tests ≥80%; lint/build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
