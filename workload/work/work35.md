# work35 — fix: payment-orchestrator rejects payable (PENDING) PayLinks

> **Seeded (bug)** — discovered during work06 live e2e. Expand with `/work 35` when picked up.

- **Status:** done · **Owner:** chain-engineer/service-builder · **Depends on:** 01, 02 · **Flow:** [flow35](../flow/flow35.md)
- **Phase:** 1 / MVP — integration defect on the core MPesa payment path.

## Problem
In the integrated stack (`docker compose --profile e2e`, the default `PAYLINK_CHAIN_SUBMIT_ENABLED=true`),
a freshly-created PayLink is returned with status **`PENDING`**: paylink-service inserts the row as
`CREATED` then flips it to `PENDING` once the on-chain `TxCreatePayLink` submit succeeds
(`paylink-service/app/domain/service.py:122`). But payment-orchestrator's `Initiate` only accepts a
PayLink whose status is exactly `CREATED` (`payment-orchestrator/internal/domain/service.go:90`),
returning **409 `PAYLINK_NOT_PAYABLE`** ("PayLink is PENDING and cannot accept a new payment") otherwise.

**Net effect:** `POST /v1/payments` can *never* succeed for a normally-created PayLink in the
integrated stack — the only payable window (`CREATED`) closes before the create call even returns.
Found via the work06 SDK live e2e (the SDK correctly surfaced the 409 as a typed `ConflictError`).
Settlement via the MPesa adapter → proof-validator path is unaffected (it settles on-chain directly),
which is why work04's e2e still passes.

## In scope
- Make the orchestrator treat a live, unsettled PayLink as payable — accept **`CREATED` and `PENDING`**
  (the off-chain "submitted, awaiting quorum" state), while still rejecting terminal states
  (`VERIFIED`/`CANCELLED`/`FAILED`/`EXPIRED`). Confirm against the paylink-service `OffChainStatus`
  enum + the chain PayLink FSM so the set is correct and not divergent.
- Keep the lifecycle projection (`internal/lifecycle`) and anti-replay guards intact.
- Add an integration test that creates a PayLink through paylink-service (submit enabled) and then
  initiates a payment successfully.

## Out of scope
- Changing the paylink-service create→PENDING transition (it is correct).
- The full payment→settlement lifecycle advance (already handled by the WS subscriber on
  `paylink.verified`).

## Invariants that apply
- **A.7 anti-replay** (one payment per PayLink; don't weaken `PAYMENT_EXISTS`), **FSM non-divergence**
  (orchestrator lifecycle stays a projection of the on-chain PayLink FSM — see work02 notes).

## Acceptance criteria
- [x] `POST /v1/payments` succeeds for a PayLink created via paylink-service with chain-submit enabled.
- [x] Terminal/!payable states still rejected with `PAYLINK_NOT_PAYABLE` (or the correct code).
- [x] Integration test covers create(submit)→initiate; orchestrator coverage stays ≥80%.
- [x] Invariant audit + `/code-review` clean.

## Notes / log
- 2026-06-12 — **done.** `Initiate` now gates on the payable set `{CREATED, PENDING}`
  (`payment-orchestrator/internal/domain/service.go` `payableStatuses`); terminal states keep 409
  `PAYLINK_NOT_PAYABLE`. Unit tests: PENDING initiate succeeds + a terminal-state rejection table
  (domain 88.1% cov). **Live-verified** on `docker compose --profile e2e` (fresh volumes, the
  ADR-015-hardened chain): create → 201 `PENDING` → `POST /v1/payments` → **201 AWAITING_PAYMENT**
  (was 409); cancelled PayLink → 409 `PAYLINK_NOT_PAYABLE`; duplicate initiate → 409 `PAYMENT_EXISTS`
  (A.7 intact); `make e2e` full settle (charge → callback → VERIFIED) still green. Invariant audit
  8/8 PASS; `/code-review` clean on this fix.
