# work15 — event bus + domain event catalog (Kafka/SQS)

> **Seeded** — expand with `/work 15` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Kafka/SQS + shared client libs · **Depends on:** — · **Flow:** [flow15](../flow/flow15.md)
- **Phase:** 1 / MVP (cross-cutting) · **Spec:** backendfeatures.md event taxonomy (ADR-004)

## Goal
The asynchronous backbone: a Kafka/SQS transport (ADR-004) plus a shared **domain event
catalog** (the `backendfeatures.md` stream/subject taxonomy as the logical model) and thin
publish/consume client libs for Python and Go services.

## In scope
- Choose + stand up the transport locally (Kafka or SQS via docker-compose) per ADR-004.
- Document the domain event catalog (e.g. `paylink.*`, `payment.*`, `chain.*`, `merchant.*`, …)
  as logical names mapped to topics/queues.
- Publish/consume helper libs (Python + Go): JSON envelope, correlation id, at-least-once.
- A `chain-event-mirror` design: subscribe lVM WS datastream → republish `chain.*` events.

## Out of scope
- NATS JetStream (recorded as a later proposal in ADR-004).
- Per-stream retention/replication tuning (Phase 2).

## Invariants that apply
- Consumers are idempotent (pair with work17); non-custodial; no secrets in event payloads.

## Reuse first
- The lVM WebSocket datastream (`paylink-chain/internal/datastream`) for the chain mirror;
  the event kinds in `paylink-chain/internal/events/event.go`.

## Acceptance criteria
- [ ] Transport runs locally; a Python and a Go service can publish + consume a test event.
- [ ] Event catalog documented (logical name → topic/queue) and referenced by services.
- [ ] chain-event-mirror republishes lVM events to `chain.*`.
- [ ] Tests for the client libs; passes the Infra/CI checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Full stack": publish an event from one service, observe
it consumed by another; observe a `chain.*` event after a node event.
