# Frontend Backlog — master tracker & coverage matrix

Every frontend work item, its paired flow, group, the `frontendfeature.md` surface (§) / `feNN` id it
implements, the **backend** work item it consumes (BE), phase, dependencies, and status. **This is the
entry point for executing the premium UI.** Pick an item whose phase is current and whose dependencies
are `done`, then run `/work <nn>` against this subtree.

Status: `todo` · `in-progress` · `blocked` · `done` · `seeded`. This tree executes the `feNN` backlog
in [`../../frontendfeature.md`](../../frontendfeature.md) §6; "BE" columns reference the backend
[`../backlog.md`](../backlog.md). Backend **work01–14 are done**, so every FE-Phase-1 item below is
buildable today.

> **Scope:** the full premium web app in `linkmint-frontend/` — design system + system UX (errors,
> motion, loading, notifications) + per-feature pages/modals for backend work01–14 + cross-cutting
> (dark mode, responsive, i18n, a11y, command palette, tests). Mobile (Flutter) and Phase-2/3 feature
> UIs (settlements, analytics, refunds, invoices, wallet) are seeded (25–30), expanded when their
> backends land.

## FE-Phase 1 — Foundation & System UX

| # | Work / Flow | Group | § / feNN | BE | Depends on | Status |
|---|-------------|-------|----------|----|------------|--------|
| 01 | [work01](work/work01.md) / [flow01](flow/flow01.md) — Design System & Theme (Ivory Premium) | Foundation | §2 / fe01 | — | — | **done** |
| 02 | [work02](work/work02.md) / [flow02](flow/flow02.md) — App Shell & Navigation | Foundation | §1 / fe02 | — | 01 | **done** |
| 03 | [work03](work/work03.md) / [flow03](flow/flow03.md) — Core Component Library | Foundation | §2.5 | — | 01,02 | **done** |
| 04 | [work04](work/work04.md) / [flow04](flow/flow04.md) — Error & Feedback System | System UX | §1 / F.5 | — | 03 | **done** |
| 05 | [work05](work/work05.md) / [flow05](flow/flow05.md) — Motion System (animations & transitions) | System UX | §2.4 | — | 01,03 | **done** |
| 06 | [work06](work/work06.md) / [flow06](flow/flow06.md) — Loading, Empty & Skeleton States | System UX | §1 | — | 03 | **done** |
| 07 | [work07](work/work07.md) / [flow07](flow/flow07.md) — Notifications & Toasts System | System UX | §1 | — | 03,04 | todo |

## FE-Phase 1 — Enabler

| # | Work / Flow | Group | § / feNN | BE | Depends on | Status |
|---|-------------|-------|----------|----|------------|--------|
| 08 | [work08](work/work08.md) / [flow08](flow/flow08.md) — SDK Expansion (identity/merchant/compliance/admin/audit) | Enabler | §4 / fe-sdk | 09,10,11,12,13 | — | todo |

## FE-Phase 1 — Feature pages & modals (backend work01–14)

| # | Work / Flow | Group | § / feNN | BE | Depends on | Status |
|---|-------------|-------|----------|----|------------|--------|
| 09 | [work09](work/work09.md) / [flow09](flow/flow09.md) — Auth (login/register/forgot/MFA) | Feature | §3.2 | 09 | 03,04,08 | done |
| 10 | [work10](work/work10.md) / [flow10](flow/flow10.md) — Account & Security (profile/sessions/API keys/orgs) | Feature | §3.2 | 09 | 08,09 | done |
| 11 | [work11](work/work11.md) / [flow11](flow/flow11.md) — PayLinks Management (list/create modal/detail/cancel/QR) | Feature | §3.3 / fe04 | 01 | 03,04 | todo |
| 12 | [work12](work/work12.md) / [flow12](flow/flow12.md) — Payments (list/detail/timeline) | Feature | §3.3 | 02 | 03 | todo |
| 13 | [work13](work/work13.md) / [flow13](flow/flow13.md) — Public Resolve & Pay (payer) | Feature | §3.1 / fe05 | 01,02,04 | 03,04,05 | todo |
| 14 | [work14](work/work14.md) / [flow14](flow/flow14.md) — Merchant Onboarding (KYB stepper) | Feature | §3.3 / fe09 | 10 | 03,08 | todo |
| 15 | [work15](work/work15.md) / [flow15](flow/flow15.md) — Compliance & KYC (status/flow/flags) | Feature | §3.2–§3.3 | 12 | 08,09 | todo |
| 16 | [work16](work/work16.md) / [flow16](flow/flow16.md) — Admin Console (search/drill-down/audit) | Feature | §3.4 / fe07 | 11,13 | 08,09 | todo |
| 17 | [work17](work/work17.md) / [flow17](flow/flow17.md) — Developer Portal (keys/quickstart/docs) | Feature | §3.5 / fe08 | 09 | 08,09 | todo |
| 18 | [work18](work/work18.md) / [flow18](flow/flow18.md) — Merchant Dashboard Overview (flagship) | Feature | §3.3 / fe03 | 01 | 03 | **done** |

## FE-Phase 2 — Cross-cutting & polish

| # | Work / Flow | Group | § / feNN | BE | Depends on | Status |
|---|-------------|-------|----------|----|------------|--------|
| 19 | [work19](work/work19.md) / [flow19](flow/flow19.md) — Theming & Dark Mode | Polish | §2.2 / fe14 | — | 01 | todo |
| 20 | [work20](work/work20.md) / [flow20](flow/flow20.md) — Responsive & Mobile | Polish | §2.7 | — | 02,03 | todo |
| 21 | [work21](work/work21.md) / [flow21](flow/flow21.md) — Internationalization (i18n) & Localization | Polish | fe14 | — | 03 | todo |
| 22 | [work22](work/work22.md) / [flow22](flow/flow22.md) — Accessibility Audit & Hardening (WCAG AA) | Polish | §2.7 / F.6 | — | 03–18 | todo |
| 23 | [work23](work/work23.md) / [flow23](flow/flow23.md) — Command Palette & Global Search (⌘K) | Polish | §1 | — | 02 | todo |
| 24 | [work24](work/work24.md) / [flow24](flow/flow24.md) — Component Tests + Storybook + Visual Regression | Polish | §7 | — | 03 | todo |

## FE-Phase 2/3 — Seeded (await backend; expand with `/work <nn>`)

| # | Work / Flow | Group | § / feNN | BE | Phase | Status |
|---|-------------|-------|----------|----|-------|--------|
| 25 | [work25](work/work25.md) / [flow25](flow/flow25.md) — Settlements UI | Feature | §3.3 | 23 | 2 | seeded |
| 26 | [work26](work/work26.md) / [flow26](flow/flow26.md) — Analytics & Reporting UI | Feature | §3.3 | 26 | 2 | seeded |
| 27 | [work27](work/work27.md) / [flow27](flow/flow27.md) — Refunds & Disputes UI | Feature | §3.3 | 22 | 2 | seeded |
| 28 | [work28](work/work28.md) / [flow28](flow/flow28.md) — Invoices & Subscriptions UI | Feature | §3.3 | 19,31 | 2 | seeded |
| 29 | [work29](work/work29.md) / [flow29](flow/flow29.md) — Wallet & Staking + Token Send UI | Feature | §3.6 | 24,34 | 3 | seeded |
| 30 | [work30](work/work30.md) / [flow30](flow/flow30.md) — Realtime (WebSocket datastream) | System UX | §5 / fe12 | chain WS | 2 | seeded |

## Coverage vs frontendfeature.md

Every §3.x surface is covered: §3.1 Public Pay → 13; §3.2 Auth & Account → 09,10 (+15 KYC); §3.3
Merchant Dashboard → 11,12,14,18 (+25–28 seeded); §3.4 Admin → 16; §3.5 Developer → 17; §3.6 Wallet →
29 (seeded). The four **system UX** areas the product calls for each have a dedicated item: errors →
04, animations/transitions → 05, loading/empty/skeleton → 06, notifications/toasts → 07. Enterprise
cross-cutting: dark mode 19, responsive 20, i18n 21, a11y 22, command palette 23, tests 24. The
`@linkmint/sdk` gap (§4) is closed by 08, which unblocks 09/10/14/15/16/17.

## Detail level

Items **01–24** are written in full (Goal, scope fences, invariants, reuse, acceptance, verification,
flow recipe). Items **25–30** are **seeded** (goal + scope + a flow skeleton) — ready to expand with
`/work <nn>` once their backend lands. Numbers are stable IDs; execution order is the phase +
dependency columns, not numeric order.

## Adding work

`/new-work <title>` scaffolds the next pair from [`../templates/`](../templates/); add a row in the
right table. Discovered side-work becomes a new row — it never expands the active item
([`../scope.md`](../scope.md)).

## Changelog
- 2026-06-01 — Seeded the frontend workload: 24 full items (foundation + system UX + features for
  backend work01–14 + cross-cutting polish) + 6 seeded future items (25–30). 01/02/18 marked **done**
  and 03 **in-progress** to reflect the Ivory Premium foundation already shipped in `linkmint-frontend/`.
