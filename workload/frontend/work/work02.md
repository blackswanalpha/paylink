# work02 — App Shell & Navigation

- **Status:** done
- **Owner:** service-builder
- **Depends on:** 01
- **Flow:** [flow02](../flow/flow02.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §1 (surface map / IA) / `fe02`

## Goal
The authenticated app frame: a sticky sidebar + topbar that hosts every dashboard surface, with the
Ivory Premium nav model (active states, brand, account chip, non-custodial trust note).

## Why / context
Enterprise apps need one consistent chrome so navigation, page headers, and content gutters are
uniform across screens. The shell is the container every feature page (work10–18) renders into.

## In scope
- `AppShell` (sidebar + main column: topbar + scrollable content, width-constrained, guttered).
- `Sidebar` (brand, nav list with active/`Soon` states, primary "Create PayLink" CTA, trust note).
- `Topbar` (mobile brand + compact nav, desktop workspace label, account chip).
- A central nav model (`nav.ts`) with `live` flags so PLANNED routes render but don't navigate (F.7).

## Out of scope (do NOT do here)
- The mobile **drawer** nav + full responsive collapse → work20. Command palette → work23. Per-page content → the feature items.

## Invariants that apply
- **F.7 phase-honest** (PLANNED nav items marked `Soon`); **F.6** (keyboard-navigable nav, `aria-current`).

## Reuse first
- The theme tokens (work01). Built at `../../../linkmint-frontend/src/components/shell/{AppShell,Sidebar,Topbar,nav}.tsx`.

## Acceptance criteria
- [x] `AppShell` frames content with sidebar (md+) + topbar; ivory canvas; constrained content width.
- [x] Sidebar nav shows active state + `Soon` tags for unbuilt routes; "Create PayLink" CTA links to `/`.
- [x] Topbar carries mobile brand/nav + account chip; `aria-current` on the active item.
- [x] `typecheck`/`lint`/`build` green; used by the dashboard (work18).
- [x] Passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": open `/dashboard`, confirm shell + nav render; keyboard-tab the nav.

## Notes / log
- **2026-06-01 — DONE.** Shipped `shell/AppShell.tsx`, `Sidebar.tsx`, `Topbar.tsx`, `nav.ts`. Sidebar
  hidden < md (mobile uses the topbar's compact nav); full mobile **drawer** deferred to work20;
  command palette deferred to work23. Account chip is demo-only until work09/10 land real sessions.
