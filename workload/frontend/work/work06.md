# work06 — Loading, Empty & Skeleton States System

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 03
- **Flow:** [flow06](../flow/flow06.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §1 (cross-cutting patterns)

## Goal
A consistent system for the three "non-data" UI states — loading, empty, and optimistic — so no
screen shows a bare spinner or a blank panel. Content regions use skeletons; empty states guide the
next action.

## Why / context
`frontendfeature.md §1` mandates skeletons over spinners and branded empty states with a primary
action. The `Skeleton` + `EmptyState` primitives exist; this item makes them a **system**: per-surface
skeleton compositions, a standard empty-state catalog, optimistic-update helpers, and Suspense
boundaries — reused by every feature page.

## In scope
- **Skeleton compositions** per layout (metric grid, table rows, detail panel, form, list-card) built
  on the `Skeleton` primitive; a `Loadable`/`AsyncBoundary` wrapper (Suspense + error seam to work04).
- **Empty-state catalog**: standardized copy + icon + CTA per surface (no PayLinks, no payments, no
  search results, no API keys, etc.), built on `EmptyState`.
- **Optimistic-update** helper for mutate-then-reconcile (cancel a PayLink, revoke a key) with rollback on error.
- **Initial-vs-refresh** distinction (skeleton on first load, inline spinner on refresh — the dashboard pattern).

## Out of scope (do NOT do here)
- Error states → work04. Toasts → work07. The actual data fetching/hooks → feature items.

## Invariants that apply
- **F.6** (loading announced via `aria-busy`); **F.5** (empty-due-to-error defers to work04, not a fake empty).

## Reuse first
- `../../../linkmint-frontend/src/components/ui/{Skeleton,EmptyState}.tsx` (+ `MetricCardSkeleton`,
  `TableRowsSkeleton`) and the `initializing` pattern in `components/dashboard/MerchantDashboard.tsx`.

## Acceptance criteria
- [ ] Skeleton compositions exist for metric/table/detail/form/list layouts; a `Loadable`/`AsyncBoundary` wrapper ships.
- [ ] An empty-state catalog covers the core surfaces with branded copy + CTA.
- [ ] An optimistic-update helper with rollback is available and used by ≥1 mutation.
- [ ] `aria-busy` on loading regions; no bare spinners on content; `typecheck`/`lint`/`build` green.
- [ ] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": throttle the network and confirm skeletons (not
spinners); view an empty surface (fresh creator) → branded empty state; cancel a PayLink → optimistic
flip + reconcile.

## Notes / log
- Pairs with work04 (errors) and work07 (toasts) as the three legs of the "system UX" the product calls for.
