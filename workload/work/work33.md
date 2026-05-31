# work33 — dashboards (merchant / admin / mobile)

> **Seeded** — expand with `/work 33` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** TypeScript (React) + Flutter (mobile) · **Depends on:** 06, 11 · **Flow:** [flow33](../flow/flow33.md)
- **Phase:** 3 / Mainnet · **Spec:** backendfeatures.md Phase 3 (dashboards)

## Goal
Self-serve UIs beyond the MVP web flow: a merchant dashboard (PayLinks, payments, settlements,
reports), an admin console UI (over work11's APIs incl. Phase-2 mutations), and a mobile app.

## In scope
- Merchant dashboard (React) over the SDK: PayLink/payment/settlement/report views.
- Admin console UI over admin-backoffice (work11) APIs (search, views, mutations w/ dual-approval).
- Flutter mobile app for the payer/merchant flows.

## Out of scope
- New backend APIs (dashboards consume existing services/SDK).
- The basic MVP web flow (work07, already done) — this extends it.

## Invariants that apply
- TS strict / no `any`; SDK-only API access (no raw fetch); non-custodial UX (no fund custody in UI).

## Reuse first
- The JS SDK (work06) + SDK suite (work32); work07 web app; admin APIs (work11); the error envelope.

## Acceptance criteria
- [ ] Merchant dashboard surfaces PayLinks/payments/settlements/reports via the SDK.
- [ ] Admin console drives work11 APIs (with dual-approval on mutations); mobile app covers core flows.
- [ ] Handles loading/error states + the standard error envelope.
- [ ] Passes the App checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "App" + "Full stack": drive each surface against the local
stack; `/run` to launch/screenshot.
