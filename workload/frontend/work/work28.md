# work28 — Invoices & Subscriptions UI

> **Seeded** — expand with `/work 28` when picked up (await backend [work19](../../work/work19.md)/[work31](../../work/work31.md)).

- **Status:** seeded · **Owner:** service-builder · **Depends on:** 03,08,11 · backend **19,31** (invoice / subscriptions) · **Flow:** [flow28](../flow/flow28.md)
- **Phase:** FE-2 · **Implements:** [frontendfeature.md §3.3](../../../frontendfeature.md) (Invoices — PLANNED)

## Goal
Multi-line invoice creation + send, and recurring-subscription management — extending PayLinks into
itemized billing and recurring revenue.

## In scope
- Invoice builder (line items, totals, due date) → shareable invoice (extends the PayLink share/QR from work11); invoice list + status.
- Subscription plans + customer subscriptions (create/pause/cancel), upcoming-charge view.

## Out of scope
- The invoice/subscription backend (work19/31).

## Invariants that apply
- **F.1 SDK-only**, **F.2 non-custodial**, **F.5**, **F.6**, **F.7**.

## Acceptance criteria
- [ ] Invoice builder creates + sends an invoice; subscription create/pause/cancel works via the SDK.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack" once backend work19/31 + their SDK resources exist.

## Notes / log
- Blocked on backend work19 (invoice) / work31 (subscriptions). Builds on the work11 PayLink share.
