# work02 — payment-orchestrator (lifecycle coordination)

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 01
- **Flow:** [flow02](../flow/flow02.md)
- **Phase:** MVP (see [scope.md](../scope.md))

## Goal
Build `linkmint-backend/payment-orchestrator` — the service that coordinates a PayLink's payment
lifecycle: from "awaiting payment" through rail callback / proof receipt to "settled", driving
state transitions and reacting to chain events.

## Why / context
PayLink Service owns the record; the **orchestrator owns the flow**. It is the conductor that
ties together the PayLink record, the chosen rail (via adapters/proof-validator), and the
on-chain settlement, exposing a coherent payment lifecycle to clients (`../../system.md`,
`../../backendfeatures.md` "Payment Orchestrator"). It does not itself verify proofs (work03)
or hold funds.

## In scope
- Service skeleton in **Go/chi** (ADR-003) per the Go service conventions in
  [standard.md](../standard.md): env config, slog logging, health/readiness/metrics; owns the
  `payment` Postgres schema; `Idempotency-Key` on initiate.
- A payment lifecycle state machine mirroring the PayLink FSM in
  `paylink-chain/internal/fsm` (no divergent states — reflect on-chain truth).
- Orchestration API, `/v1/payments`: initiate a payment for a PayLink, query payment status.
- Consume lVM chain events (via the WebSocket datastream / event stream) to advance lifecycle
  state on settlement / cancellation.
- Idempotent handling — a replayed event/callback must not double-advance (aligns with A.7).
- Tests (unit + integration) ≥80%; Dockerfile + docker-compose entry.

## Out of scope (do NOT do here)
- Proof signature verification and chain broadcast → work03.
- Rail-specific callback handling → work04 (adapter).
- CRUD of the PayLink record itself → work01.
- Retries/escrow/refund policy beyond basic lifecycle (escrow is deferred).

## Invariants that apply
- **A.1 Non-custodial** — orchestrates state, never funds.
- **A.3 PoV** — settlement truth comes from chain quorum; the orchestrator reacts to it,
  it does not decide settlement.
- **A.7 Anti-replay** — idempotent on duplicate events/callbacks.

## Reuse first
- PayLink/Validator FSM transitions in `paylink-chain/internal/fsm`.
- The chain event kinds in `paylink-chain/internal/events/event.go` and the datastream
  WebSocket protocol in `paylink-chain/internal/datastream`.
- The Go/chi service conventions in [standard.md](../standard.md) (this is the reference Go
  service template for work03/20/23/24/27).
- paylink-service's `/v1/paylinks` for record lookups.

## Acceptance criteria
- [x] `POST /v1/payments` initiates a payment lifecycle for an existing PayLink.
- [x] `GET /v1/payments/:id` returns lifecycle status consistent with on-chain state.
- [x] The service subscribes to chain events and advances lifecycle on settle/cancel.
- [x] Duplicate events/callbacks are idempotent (no double transition).
- [x] Lifecycle states do not diverge from the on-chain PayLink FSM.
- [x] Tests ≥80%; lint + build clean; docker-compose entry healthy.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack": run the stack,
initiate a payment, emit/observe a settlement event from the node, confirm the lifecycle
advances exactly once.

## Notes / log
- Treat the on-chain FSM as the single source of truth; the orchestrator is a projection +
  driver, never an authority on settlement.
- 2026-06-12 — audit re-verified: build/vet/tests green (15 pkgs, domain 88% cov). work35 fixed — `Initiate` now accepts `CREATED`+`PENDING`, so the initiate criterion holds in the integrated stack (it 409d before). Status header synced, boxes ticked.
