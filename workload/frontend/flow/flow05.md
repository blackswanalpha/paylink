# flow05 — Motion System (execution recipe)

**Work item:** [work05](../work/work05.md) · **Goal recap:** a cohesive, reduced-motion-safe animation language (routes, overlays, lists, numbers, micro-interactions).

## Pre-flight
- [ ] Read [work05](../work/work05.md), [frontendfeature.md §2.4](../../../frontendfeature.md), the motion tokens in `theme/system.ts`, the `lm-pulse` keyframe in `globals.css`.
- [ ] Confirm work01 + work03 are usable (overlays to animate).
- [ ] Set work05 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory animation targets (routes, overlays, lists, metrics, micro) + the existing tokens/keyframes | **Explore** | motion map |
| 2 | Choose the approach (framer-motion vs CSS-only) + design the wrappers/hooks; record in decisions.md | **Plan** | decision + API |
| 3 | Implement `useReducedMotion`, `PageTransition`, overlay motion (into work03), list stagger, count-up, micro-interactions | **service-builder** | motion system |
| 4 | Verify reduced-motion no-ops everywhere; SSR-safe | **service-builder** | reduced-motion pass |
| 5 | Review a11y + bundle | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Drive routes/overlays/dashboard; toggle OS reduce-motion | `/verify` | observed + stills when reduced |

## Done
- [ ] Acceptance criteria in [work05](../work/work05.md) met; **App** checklist complete; ADR for the motion lib filed; status `done`.
