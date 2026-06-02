# work18 — Merchant Dashboard Overview (flagship)

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 03 · backend [work01](../../work/work01.md)
- **Flow:** [flow18](../flow/flow18.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §3.3 (Overview) / `fe03`

## Goal
The merchant's home: at-a-glance metrics (total settled, active, pending), a recent-activity sparkline,
and a recent-PayLinks table — the flagship screen that exercises the whole design system on **live**
work01 data.

## Why / context
The overview is the first thing a merchant sees and the reference implementation proving the Ivory
Premium system end-to-end. Built last session as the flagship.

## In scope
- `/dashboard` in the app shell: `MetricCard`s (total settled / active / pending / total), a recent-
  activity `Sparkline`, and a recent-PayLinks `DataTable`, with a "Create PayLink" CTA.
- Aggregates derived **client-side** from `client.paylinks.list` (count by status, sum settled,
  sparkline by `created_at`); skeletons on first load; empty state when no PayLinks.

## Out of scope (do NOT do here)
- Full PayLink management (list/create/cancel) → work11. True analytics (revenue series/conversion) →
  work26 (PLANNED, marked in-UI). Settlements → work25.

## Invariants that apply
- **F.1 SDK-only**, **F.5**, **F.6**, **F.7** (richer analytics marked PLANNED, not faked).

## Reuse first
- `client.paylinks.list` (work06 SDK); `AppShell` (work02); `MetricCard`/`Sparkline`/`DataTable`/
  `EmptyState`/`Skeleton`/`StatusPill`/`AmountDisplay`/`AddressChip` (built); the `usePayLinks` hook.

## Acceptance criteria
- [x] `/dashboard` renders metrics, sparkline, and a recent-PayLinks table from live data.
- [x] Skeletons on first load; empty state when none; correct `StatusPill`s + amounts.
- [x] Richer analytics marked PLANNED (work26); `typecheck`/`lint`/`build` green.
- [x] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App" + "Full stack": seed PayLinks, open `/dashboard`,
confirm metrics/sparkline/table render from the live gateway.

## Notes / log
- **2026-06-01 — DONE.** Shipped `linkmint-frontend/src/components/dashboard/MerchantDashboard.tsx` +
  `src/hooks/usePayLinks.ts` + `app/dashboard/page.tsx` (server mints JWT via `lib/jwt.ts`). Verified
  live: 3 seeded PayLinks listed via the same-origin `/v1` proxy; metrics computed; build green. Richer
  analytics deferred to work26.
