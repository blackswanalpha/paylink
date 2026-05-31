# work22 — refund-dispute-service (refunds + chargebacks)

> **Seeded** — expand with `/work 22` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 02, 23 · **Flow:** [flow22](../flow/flow22.md)
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
- [ ] Refund request → approve → rail reversal → COMPLETED (full + partial).
- [ ] Dispute intake (HMAC) → evidence upload (window) → submit → resolution; clawback on loss.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
