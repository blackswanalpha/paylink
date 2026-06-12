# work15 — event bus + domain event catalog (Kafka/SQS)

> **Seeded** — expand with `/work 15` when picked up.

- **Status:** done · **Owner:** service-builder · **Stack:** Kafka via Redpanda + shared client libs (ADR-011) · **Depends on:** — · **Flow:** [flow15](../flow/flow15.md)
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
- [x] Transport runs locally; a Python and a Go service can publish + consume a test event.
- [x] Event catalog documented (logical name → topic/queue) and referenced by services.
- [x] chain-event-mirror republishes lVM events to `chain.*`.
- [x] Tests for the client libs; passes the Infra/CI checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Full stack": publish an event from one service, observe
it consumed by another; observe a `chain.*` event after a node event.

## Notes / log
- **Done 2026-06-02.** Transport = Kafka via **Redpanda** (ADR-011). Deliverables: `workload/catalog.md`
  (event catalog); `linkmint-backend/eventbus-go` (franz-go, 81.8% cov) + `eventbus-python`
  (`linkmint_eventbus`, aiokafka, 99% cov) — byte-identical envelope, golden-tested; `chain-event-mirror`
  (Go service, 96% of tested pkgs) republishing lVM `/ws` events as `chain.*`; Redpanda + topic-init in
  docker-compose.
- **Producers retrofitted:** paylink/identity/merchant/compliance (Python, outbox-drain relay) +
  payment-orchestrator/proof-validator (Go, inline publish). **Consumers:** notification/identity/
  merchant/compliance (Python lifespan bus-consumer → existing `handle()`). All env-gated + lazily
  imported (default `log`/disabled → existing suites untouched; all service unit suites still green).
- **Live-verified:** paylink (kafka mode) drained 12 outbox rows → canonical envelopes on the `paylink`
  topic; notification consumed all (group lag 0); wire format confirmed; at-least-once held on a
  malformed event (offset uncommitted → redelivered). eventbus-go/python/mirror integration tests pass
  on Redpanda.
- **Follow-ups** (ADR-011 + backlog): Go transactional-outbox; paylink consuming `chain.*` (reconcile
  from bus); Go inbound consumers (orchestrator `paylink.requested`, proof-validator
  `payment.proof_received`, audit-log `intake.Source`); a dead-letter policy for poison messages; an
  `admin` topic for `admin.override.*`; flipping the remaining services to kafka mode in compose
  (Dockerfile→repo-root context + env; paylink/notification are the worked examples).
- 2026-06-12 — audit re-verified: suites fresh-green against the ADR-015 chain (eventbus-go 84.0%,
  eventbus-python 99.3%, chain-event-mirror 96.2%; mirror's event shapes unaffected by the hardening);
  boxes ticked. The "Go inbound consumers" follow-up got its first instance: escrow-manager (work20)
  consumes `chain.paylink.verified`.
