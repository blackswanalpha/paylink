# work13 — Public Resolve & Pay (payer)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03, 04, 05 · backend [work01](../../work/work01.md), [work02](../../work/work02.md), [work04](../../work/work04.md)
- **Flow:** [flow13](../flow/flow13.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.1 (Public Resolve & Pay) / `fe05`

## Goal
The payer-facing, **public** (no-account) pay experience: open a PayLink, see who's requesting what,
choose a rail, pay (MPesa STK / instructions), and watch it settle live — ending in a receipt. The
productized form of the work07 wizard.

## Why / context
This is the conversion surface (PRD Journey 2) and the most trust-sensitive screen — it must feel
safe and premium. PayLink resolve is a public endpoint; payment initiate + settlement poll are LIVE.
Non-custodial: the UI only shows instructions/initiates; **no PIN/PAN is ever entered** (F.2).

## In scope
- `/pay/[plId]`: resolve via `client.paylinks.get` (public) → merchant + `AmountDisplay` + expiry +
  status, with trust framing (non-custodial badge, secure context).
- **Method picker** (rail label + icon, F.3); for MPesa → STK push via `client.payments.initiate` +
  `PayInstructions` (paybill/account copy); graceful `PAYLINK_NOT_PAYABLE` note (the work35 gap).
- **Live settlement** (poll `paylinks.get` to terminal) with motion (work05) → success **receipt**
  (chain tx hash, verified time) or a calm failed/expired/already-settled screen.
- Distinct, branded states for not-found / expired / already-settled / cancelled.

## Out of scope (do NOT do here)
- Card/bank/crypto rails (PLANNED adapters) — show as `Soon`. Merchant-side management → work11. Real-PIN capture (never — F.2).

## Invariants that apply
- **F.2 non-custodial** (no fund/PIN/PAN capture — the hard fence), **F.3 rail-agnostic**, **F.1 SDK-only**, **F.5**, **F.6**, **F.7** (PLANNED rails marked).

## Reuse first
- `client.paylinks.get` + `client.payments.initiate` (work06 SDK); the work07 `PayInstructions` +
  `SettlementStatus` + `useInitiatePayment` + `useSettlementStatus`
  (`../../../linkmint-frontend/src/components/{PayInstructions,SettlementStatus}.tsx`,
  `src/hooks/*`); `QRBlock`/`AmountDisplay`/`StatusPill` (built); motion (work05); errors (work04).

## Acceptance criteria
- [ ] `/pay/[plId]` resolves a real PayLink (public) and shows merchant/amount/expiry/status with trust framing.
- [ ] MPesa method initiates a charge + shows instructions; settlement polls to VERIFIED → receipt with tx hash.
- [ ] not-found/expired/already-settled/cancelled each render a distinct branded screen; PLANNED rails marked `Soon`.
- [ ] No PIN/PAN field anywhere (F.2); `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": open a created PayLink's `/pay/[plId]`,
pay via MPesa (Daraja stub), watch PENDING→VERIFIED → receipt; visit an expired/unknown id → branded state.

## Notes / log
- Folds the proven work07 wizard components into a public route. Non-custodial is the non-negotiable fence here.
