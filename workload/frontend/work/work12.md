# work12 — Payments (list / detail / lifecycle timeline)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03 · backend [work02](../../work/work02.md)
- **Flow:** [flow12](../flow/flow12.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.3 (Payments)

## Goal
The payments surface: a list of payments and a detail view with a lifecycle timeline
(`AWAITING_PAYMENT → SETTLED | FAILED | CANCELLED`), linked to the PayLink each settles.

## Why / context
Merchants need to see money movement, not just PayLinks. payment-orchestrator (backend work02) owns
payment lifecycle; this surfaces it with a premium timeline and cross-links to the PayLink (work11).

## In scope
- **List** (`/dashboard/payments`): `DataTable` of payments (rail label, status, timestamps), filter by status.
- **Detail** (`/dashboard/payments/[id]`): `client.payments.get` → a vertical **timeline** of the
  lifecycle with `PaymentStatusPill`, the opaque `rail` label + icon, and a link to the PayLink.
- Live status: poll to terminal (reuse the settlement-poll pattern) until work30 realtime lands.

## Out of scope (do NOT do here)
- Initiating a payment (payer flow) → work13. Refunds → work27 (seeded). Settlement batches → work25 (seeded).

## Invariants that apply
- **F.1 SDK-only**, **F.3 rail-agnostic** (rail is an opaque label, no rail-specific fields), **F.5**, **F.6**.

## Reuse first
- `client.payments.{get}` (and the list once exposed — note in §4 if the list isn't in the SDK yet);
  `DataTable` (work03); `PaymentStatusPill` (built); the `useSettlementStatus` poll pattern
  (`../../../linkmint-frontend/src/hooks/useSettlementStatus.ts`).

## Acceptance criteria
- [ ] Payments list renders via the SDK with status filter; rows link to detail.
- [ ] Detail shows a lifecycle timeline with the correct `PaymentStatusPill` and a link to the PayLink.
- [ ] Status updates live (poll) to terminal; rail shown as an opaque label only (F.3).
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": drive a payment to SETTLED via the
adapter and watch the timeline advance; confirm no rail-specific fields leak into the view.

## Notes / log
- If `payments.list` isn't in the SDK, file it under work08 (§4) rather than raw-fetching (F.1).
