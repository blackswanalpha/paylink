# work19 — invoice-subscription (invoices)

> **Seeded** — expand with `/work 19` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 01 · **Flow:** [flow19](../flow/flow19.md)
- **Phase:** 2 / Beta (invoices; subscriptions are work31) · **Spec:** backendfeatures.md §2.6

## Goal
Multi-line invoices that aggregate to a single PayLink, with the DRAFT→OPEN→PAID|VOID|OVERDUE
lifecycle. Recurring subscriptions are deferred to work31 (Phase 3).

## In scope
- `/v1/invoices` (create with lines), `/finalize`, `/void`; aggregate lines → one PayLink.
- Lifecycle: DRAFT → OPEN → PAID | VOID | OVERDUE; finalize one-way; void blocked after partial pay.
- Owns `invoice` schema; consumes `chain.paylink.verified` (mark paid); publishes `invoice.*`.

## Out of scope
- Subscriptions / recurring billing / dunning / proration (work31, Phase 3).

## Invariants that apply
- Non-custodial; rail-agnostic; settlement truth from chain (`chain.paylink.verified`).

## Reuse first
- work01 paylink-service (`/v1/paylinks`) to create the aggregated PayLink; event bus (work15).

## Acceptance criteria
- [ ] Create multi-line invoice → finalize → single PayLink → pay → invoice PAID.
- [ ] Finalize one-way; void blocked after partial payment; OVERDUE on due date.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
