# work19 — Theming & Dark Mode

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 01
- **Flow:** [flow19](../flow/flow19.md)
- **Phase:** FE-2
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2.2 (dark seam) / `fe14`

## Goal
A first-class dark theme with full semantic-token parity, a theme toggle (light / dark / system), and a
persisted preference — completing the dark seam scaffolded in the design system.

## Why / context
The Ivory Premium system was built light-first with a dark seam (`frontendfeature.md §2.2`). Enterprise
users expect dark mode; because everything reads semantic tokens (work01), this is a token swap + a
toggle, not a per-component rewrite.

## In scope
- `_dark` values for every semantic token (bg/fg/border/accent/gold/status) — dark parity that keeps
  AA contrast; a dark ivory→ink inversion that stays warm and premium.
- A theme toggle (light/dark/system) in the topbar; persistence (localStorage + `prefers-color-scheme`);
  SSR-safe (no flash — set `color-scheme` + an early class).
- Audit each built surface (shell, dashboard, components) in dark.

## Out of scope (do NOT do here)
- New components. Per-tenant theming/white-label (Phase 3).

## Invariants that apply
- **F.6** (AA contrast in **both** modes; no flash), **F.7**.

## Reuse first
- The semantic-token structure in `../../../linkmint-frontend/src/theme/system.ts` (add `_dark`); the
  `color-scheme` handling in `globals.css`; Chakra v3 color-mode utilities.

## Acceptance criteria
- [ ] Every semantic token has a dark value meeting AA; all built surfaces render correctly in dark.
- [ ] A light/dark/system toggle persists and is SSR-safe (no flash-of-wrong-theme).
- [ ] `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": toggle modes across dashboard/shell/components;
reload (persists, no flash); run a contrast check in dark.

## Notes / log
- Cheap because of the token discipline in work01 — keep it a token swap, not a component fork.
