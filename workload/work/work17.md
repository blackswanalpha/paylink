# work17 — idempotency framework (shared)

> **Seeded** — expand with `/work 17` when picked up.

- **Status:** done · **Owner:** service-builder · **Stack:** shared Python/Go middleware + Redis · **Depends on:** 15 · **Flow:** [flow17](../flow/flow17.md)
- **Phase:** 1 / MVP (cross-cutting) · **Spec:** backendfeatures.md §"Idempotency"

## Goal
A shared idempotency mechanism so every state-mutating endpoint and every event consumer is
safe to retry — the application-layer complement to the chain's on-chain anti-replay (A.7).

## In scope
- `Idempotency-Key` HTTP middleware (Python + Go): Redis `idem:<service>:<key>`, 24h TTL,
  returns the cached response on replay.
- Consumer-side idempotency helper (dedupe by event id / proof_hash).
- Guidance doc on per-flow uniqueness (proof_hash UNIQUE, refund_id/dispute_id UNIQUE, etc.).

## Out of scope
- Service-specific business keys (each service declares its own uniqueness constraints).

## Invariants that apply
- **Anti-replay (A.7)** at the app layer; defers to the on-chain proof-hash check as source of truth.

## Reuse first
- Redis (from docker-compose); the proof_hash semantics in proof-validator (work03);
  the event bus (work15) consumer contracts.

## Acceptance criteria
- [x] Middleware caches + replays responses by `Idempotency-Key` (Redis, 24h TTL).
- [x] Consumer helper dedupes repeated events; double-delivery causes no double effect.
- [x] Adopted by at least one Python and one Go service; tests ≥80%.
- [x] Passes the relevant [definition-of-done.md](../definition-of-done.md) checklist(s).

## Verification
[verification.md](../verification.md) → "Backend service": send a mutating request twice with the
same key; confirm single effect + identical response; redeliver an event, confirm no double effect.

## Notes / log
- Shipped as the sibling libs `linkmint-backend/idempotency-go` + `idempotency-python`: HTTP
  `Idempotency-Key` store (`idem:<service>:<route>:<key>`, 24h TTL, replay + body-mismatch/in-flight
  conflict) adopted by all 9 services (4 Go via `replace`, 5+ Python via pip); consumer-side
  `RedisDedupe` (best-effort short-circuit) + `DbDedupe` (durable, caller's tx).
- 2026-06-12 — audit: header was stale (`todo`); flipped to done. Consumer dedupe verified wired:
  `RedisDedupe` live in the notification-service, invoice-subscription and fee-pricing-service bus
  consumers (`app/busconsumer/run.py`) over idempotent domain upserts; `DbDedupe` gets its first
  transactional wiring in escrow-manager's `chain.paylink.verified` consumer (work20). Suites
  fresh-green: idempotency-go 88.6%, idempotency-python 97.6%; boxes ticked.
