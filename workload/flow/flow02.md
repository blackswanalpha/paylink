# flow02 — payment-orchestrator (execution recipe)

**Work item:** [work02](../work/work02.md) · **Goal recap:** the conductor of the payment
lifecycle — reacts to chain events, advances state, idempotent, reflects on-chain FSM.

## Pre-flight
- [ ] Read [work02](../work/work02.md), [rules.md](../rules.md).
- [ ] Confirm work01 is `done` in [backlog.md](../backlog.md).
- [ ] Set work02 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the PayLink/Validator FSM, event kinds, and datastream protocol on the chain | **Explore** (`internal/fsm`, `internal/events`, `internal/datastream`) | the canonical state set + event contract |
| 2 | Design the lifecycle SM as a projection of the on-chain FSM + the `/v1/payments` API + idempotency strategy | **Plan** | design doc |
| 3 | Scaffold the Go/chi skeleton (chi router, env config, slog, health/readiness/metrics) | `/scaffold-service` | `linkmint-backend/payment-orchestrator/` skeleton |
| 4 | Implement the datastream/event subscriber + idempotent lifecycle transitions | **service-builder** | event consumer |
| 5 | Implement `/v1/payments` initiate + status, calling paylink-service for records | **service-builder** | endpoints |
| 6 | Unit + integration tests (incl. duplicate-event idempotency); ≥80% | **service-builder** | passing tests |
| 7 | Review vs invariants (A.1/A.3/A.7) + quality | **invariant-auditor** + `/code-review` | clean diff |
| 8 | Verify on the stack: initiate → emit settlement event → single advance | `/verify` | observed lifecycle |

## Done
- [ ] Acceptance criteria in [work02](../work/work02.md) met.
- [ ] Backend-service DoD checklist complete; work02 → `done` in [backlog.md](../backlog.md).
