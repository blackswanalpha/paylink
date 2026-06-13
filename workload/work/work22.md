# work22 — refund-dispute-service (refunds + chargebacks)

> **Done** — `linkmint-backend/refund-dispute-service` (Python/FastAPI, port 8100, `refund` schema),
> built 2026-06-13 (see [backlog.md](../backlog.md)). Two deviations, both repo-precedented seams:
> **rail reversal is instruction-only** (no rail adapter supports reversal yet — mpesa is
> STK-push-only, card/crypto/bank are work28–30) via a `RailReversalRegistry`; **clawback is a
> published `refund.clawback.requested` contract** for settlement (work23, not built), so the
> `02,23` dep is satisfied without work23 — the same seam work11 used for the audit sink before
> work13. A.6 ledger posting is OFF (work23 is the canonical writer). Escrow-dispute resolution
> (escrow-manager's DISPUTED seam) stays out of scope.

- **Status:** done · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 02, 23 · **Flow:** [flow22](../flow/flow22.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.9

## Goal
Refunds (sender/merchant-initiated) and disputes/chargebacks (rail-initiated) with evidence
collection, rail-specific reversal, and the PENDING→PROCESSING→COMPLETED and dispute lifecycles.

## In scope
- `/v1/refunds` (+approve), `/v1/disputes` (HMAC intake), `/disputes/{id}/evidence`, `/submit`.
- Rail-specific reversal (MPesa B2C, Stripe refund, crypto = new outbound, bank ACH return).
- Owns `refund` schema; consumes rail chargeback webhooks + `chain.paylink.verified`; publishes `refund.*`, `dispute.*`.
- Clawback from next payout on dispute loss (coordinates with settlement, work23).

## Out of scope
- The settlement payout mechanics themselves (work23).
- Holding funds — refunds flow via the rail/merchant, non-custodial (A.1).

## Invariants that apply
- **Non-custodial (A.1)**; rail-agnostic at the boundary; evidence windows are rail-imposed deadlines.

## Reuse first
- payment-orchestrator (work02) for payment lookups; settlement (work23) for clawback; event bus (work15).

## Acceptance criteria
- [x] Refund request → approve → rail reversal → COMPLETED (full + partial). _(reversal is
  instruction-only — `refund.reversal.instructed`; dev sweeper simulates PROCESSING→COMPLETED.)_
- [x] Dispute intake (HMAC) → evidence upload (window) → submit → resolution; clawback on loss.
- [x] Tests ≥80%; lint/build clean. _(104 unit tests, cover 94%; ruff/black/mypy clean; image builds.)_
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
