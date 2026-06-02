# work27 — Refunds & Disputes UI

> **Seeded** — expand with `/work 27` when picked up (await backend [work22](../../work/work22.md)).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 03,08 · backend **22** (refund-dispute) · **Flow:** [flow27](../flow/flow27.md)
- **Phase:** FE-2 · **Implements:** [frontendfeature.md §3.3](../../../frontendfeature.md) (Refunds/Disputes — PLANNED)

## Goal
Refund initiation + dispute lifecycle UI: request a refund on a settled payment, track dispute status,
and submit evidence.

## In scope
- Refund action on a payment (work12) → status tracking; a disputes list + detail with a lifecycle timeline + evidence upload.
- Reuse the payment detail (work12), `DataTable`, `Stepper`/timeline, evidence upload (work03/14 patterns).

## Out of scope
- The refund/dispute backend (work22). Chargeback rails internals.

## Invariants that apply
- **F.1 SDK-only**, **F.2 non-custodial**, **F.5**, **F.6**, **F.7**.

## Acceptance criteria
- [ ] Refund request + dispute lifecycle + evidence submission work via the SDK.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack" once backend work22 + its SDK resource exist.

## Notes / log
- Blocked on backend work22. Extends the payment detail (work12).
