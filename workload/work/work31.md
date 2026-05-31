# work31 — subscriptions (recurring billing; extends work19)

> **Seeded** — expand with `/work 31` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 19 · **Flow:** [flow31](../flow/flow31.md)
- **Phase:** 3 / Mainnet · **Spec:** backendfeatures.md §2.6 (subscriptions, Phase 3)

## Goal
Recurring billing on top of the invoice service: subscriptions with auto-charge on schedule,
dunning on failure, and proration/credits, with the ACTIVE↔PAUSED→CANCELLED machine.

## In scope
- `/v1/subscriptions` (+cancel/pause/resume); auto-charge on `next_charge_at`; 14-day dunning sequence; proration/credits.
- Extends the `invoice` schema (subscriptions table); consumes `payment.failed` → dunning; publishes `subscription.*`.

## Out of scope
- The base invoice flow (work19, already done).

## Invariants that apply
- Non-custodial; rail-agnostic; settlement truth from chain.

## Reuse first
- work19 invoice service (creates the per-cycle PayLink/invoice); event bus (work15); notification (work14) for dunning.

## Acceptance criteria
- [ ] Subscription auto-charges per cycle via an invoice/PayLink; pause/resume/cancel work.
- [ ] Dunning sequence on failed charge; proration on plan change.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
