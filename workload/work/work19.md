# work19 — invoice-subscription (invoices)

- **Status:** done · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 01 · **Flow:** [flow19](../flow/flow19.md)
- **Phase:** 2 / Beta (invoices; subscriptions are work31) · **Spec:** backendfeatures.md §2.19
- **Service:** `linkmint-backend/invoice-subscription/` (port 8096, `invoice` schema)

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
- [x] Create multi-line invoice → finalize → single PayLink → pay → invoice PAID.
- [x] Finalize one-way; void blocked after partial payment; OVERDUE on due date.
- [x] Tests ≥80%; lint/build clean.
- [x] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".

## Notes / log
- 2026-06-12 — audit re-verified: lifecycle + finalize→PayLink (`Idempotency-Key=invoice-<id>`) +
  `chain.paylink.verified`→PAID consumer + OVERDUE sweeper/lazy all in place; suite fresh-green
  (57 tests, 95.2%); prometheus scrape target added under work18's audit note; boxes ticked.
