# work05 — Motion System (animations & transitions)

- **Status:** todo
- **Owner:** service-builder
- **Depends on:** 01, 03
- **Flow:** [flow05](../flow/flow05.md)
- **Phase:** FE-1
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2.4 (motion)

## Goal
A cohesive motion language — route/page transitions, overlay enter/exit, list and number animation,
and micro-interactions — that makes the app feel alive and premium, **always** gated on
`prefers-reduced-motion`.

## Why / context
Motion is a core part of the "premium, luxury" feel the product targets. Done ad-hoc it's
inconsistent and inaccessible; done as a **system** (shared tokens + a few wrappers) it's uniform and
respects users who opt out. The motion tokens (durations/easing) already exist in
`theme/system.ts §2.4`; this item turns them into reusable motion components.

## In scope
- **Motion tokens** consumed from the theme (durations `fast/base/slow`, easing curve); a
  `useReducedMotion()` guard wrapping every animation.
- **Route/page transitions** (fade/slide on navigation via the App Router) and a `PageTransition` wrapper.
- **Overlay motion** for Modal/Drawer/Menu/Tooltip (enter/exit, backdrop fade) — applied through the
  work03 components so every overlay animates consistently.
- **List motion** (stagger-in, reorder, add/remove) for tables/cards; **number count-up** for
  `MetricCard` values; **skeleton shimmer** (the `lm-pulse` keyframe already in `globals.css`).
- **Micro-interactions**: button press, card hover-lift (already tokenized), copy "✓" feedback,
  success burst on settlement (champagne accent), toast slide.
- A single dependency decision (e.g. `framer-motion`) recorded in [../../decisions.md](../../decisions.md), or a
  CSS-only approach if it keeps bundle/SSR simpler — choose and document.

## Out of scope (do NOT do here)
- New components (just animate work03's). Page content. Heavy scroll-driven/3D effects (Phase 3).

## Invariants that apply
- **F.6 Accessibility** — every animation no-ops under `prefers-reduced-motion`; motion never blocks
  interaction or conveys the only signal. **F.7** — motion never fakes data/loading that isn't real.

## Reuse first
- The motion tokens + `_hover` lift in `../../../linkmint-frontend/src/theme/system.ts`; the
  `lm-pulse` keyframe + reduced-motion media query already in `app/globals.css`; the `Sparkline`/
  `MetricCard` for count-up targets.

## Acceptance criteria
- [ ] Route transitions, overlay enter/exit, list stagger, number count-up, and micro-interactions ship as reusable wrappers/hooks.
- [ ] **Every** animation is disabled under `prefers-reduced-motion` (verified).
- [ ] The motion dependency choice is recorded in `decisions.md`; bundle impact noted; SSR-safe.
- [ ] No `any`; `typecheck`/`lint`/`build` green; passes the **App** checklist + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": navigate (route transition), open a Modal/Drawer
(enter/exit), load the dashboard (count-up + stagger + shimmer); toggle OS "reduce motion" and confirm
all of it stills.

## Notes / log
- The **animation exemplar** — feature items animate via these wrappers, not bespoke transitions.
