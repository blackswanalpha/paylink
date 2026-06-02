# work11 — PayLinks Management (list / create modal / detail / cancel / QR)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03, 04 · backend [work01](../../work/work01.md)
- **Flow:** [flow11](../flow/flow11.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.3 (PayLinks) / `fe04`

## Goal
The merchant's PayLink workspace: a filterable, paginated list; a **Create** modal/wizard; a detail
**drawer**; cancel; and share (link + QR). Fully **LIVE** against work01 today.

## Why / context
Creating and managing PayLinks is the merchant's core job-to-be-done (PRD Journey 1). The work07 demo
has a one-off create form; this productizes it into a real management surface using the SDK already
covering paylinks. The exemplar **feature** item — its patterns (list+filter+paginate, create-modal,
detail-drawer, optimistic mutate) repeat across work12/14/16.

## In scope
- **List** (`/dashboard/paylinks`): `DataTable` over `client.paylinks.list` with filter (status,
  creator/receiver) + cursor "Load more"; `StatusPill`, `AmountDisplay`, `AddressChip`, created date.
- **Create**: a `Modal`/`Stepper` (refactor the work07 `CreatePayLinkForm` into a reusable modal) —
  receiver, amount (minor units), currency, expiry, usage; idempotent create; handles the **402
  KYC_REQUIRED** gate via work04 (CTA to work15).
- **Detail Drawer**: full PayLink (status, votes, chain tx hash, timestamps) + actions.
- **Cancel** (optimistic via work06, confirm Modal); **Share** (copy URL + `QRBlock`).

## Out of scope (do NOT do here)
- The payer-facing pay page → work13. Payments list → work12. Analytics → work26 (seeded). Settlement → work25.

## Invariants that apply
- **F.1 SDK-only**, **F.2 non-custodial** (no fund capture), **F.3 rail-agnostic** (no rail fields on
  the PayLink view), **F.5** (envelope incl. 402/409), **F.8** (idempotent create/cancel), **F.6**.

## Reuse first
- `client.paylinks.{list,get,create,cancel}` (work06 SDK); `DataTable`/`Modal`/`Drawer`/`Stepper`/
  `QRBlock` (work03); `StatusPill`/`AmountDisplay`/`AddressChip` (built); the work07 `CreatePayLinkForm`
  (`../../../linkmint-frontend/src/components/CreatePayLinkForm.tsx`) as the create-form basis;
  optimistic helper (work06); the dashboard `usePayLinks` hook for list/aggregation.

## Acceptance criteria
- [ ] List filters + paginates (cursor) via the SDK; rows show status/amount/ids/date.
- [ ] Create modal creates a PayLink (idempotent), shows it in the list, and handles 402 KYC gracefully.
- [ ] Detail drawer shows full state incl. chain tx hash; cancel is optimistic + confirmed; share copies URL + QR.
- [ ] No `any`; `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": create via the modal → appears in
list → open drawer → cancel (optimistic) → reconcile; share → QR renders; force a 402 (tier-0 over threshold).

## Notes / log
- **Feature exemplar.** LIVE on backend work01 now. Reuse the work07 create form rather than rewriting it.
