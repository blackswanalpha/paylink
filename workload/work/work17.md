# work17 — idempotency framework (shared)

> **Seeded** — expand with `/work 17` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** shared Python/Go middleware + Redis · **Depends on:** 15 · **Flow:** [flow17](../flow/flow17.md)
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
- [ ] Middleware caches + replays responses by `Idempotency-Key` (Redis, 24h TTL).
- [ ] Consumer helper dedupes repeated events; double-delivery causes no double effect.
- [ ] Adopted by at least one Python and one Go service; tests ≥80%.
- [ ] Passes the relevant [definition-of-done.md](../definition-of-done.md) checklist(s).

## Verification
[verification.md](../verification.md) → "Backend service": send a mutating request twice with the
same key; confirm single effect + identical response; redeliver an event, confirm no double effect.
