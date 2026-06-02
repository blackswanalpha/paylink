# work01 — Design System & Theme (Ivory Premium)

- **Status:** done
- **Owner:** service-builder
- **Depends on:** —
- **Flow:** [flow01](../flow/flow01.md)
- **Phase:** FE-1 (see [../../scope.md](../../scope.md))
- **Implements:** [frontendfeature.md](../../../frontendfeature.md) §2 / `fe01`

## Goal
Establish the **Ivory Premium** design language as a real Chakra UI v3 system — the token foundation
(color, type, space, elevation, motion) every other frontend item builds against.

## Why / context
The work07 demo ran on Chakra's `defaultSystem` (no brand). A premium, enterprise UI needs one
cohesive, tokenized language so screens are consistent and re-theming (dark mode, work19) is a
token swap, not a rewrite. This is the root dependency of the whole tree
([frontendfeature.md §2](../../../frontendfeature.md)).

## In scope
- A custom `createSystem(defaultConfig, …)` with tokens: ivory canvas/ink/hairline, emerald accent
  ramp, champagne gold, status hues; `fonts` (Fraunces display + Inter UI + mono); `radii`,
  `shadows` (soft layered); `globalCss` (bg/fg/focus-ring/selection).
- **Semantic tokens** re-mapping Chakra's `bg`/`fg`/`border` + `accent.*`, `gold.*`, and `status.*`
  (the single source `StatusPill` reads).
- Fonts loaded build-safe (Google-Fonts `<link>` + fallback stack); light-first with a dark seam.

## Out of scope (do NOT do here)
- The component library itself → work03. The shell → work02. Dark-mode toggle/persistence → work19.

## Invariants that apply
- **F.6 Accessibility** — focus ring + contrast tokens defined here; **F.7 phase-honest** (no faked surfaces).

## Reuse first
- Chakra v3 `createSystem`/`defineConfig`/`defaultConfig`. Built at
  `../../../linkmint-frontend/src/theme/system.ts`, `app/layout.tsx`, `components/ui/Provider.tsx`,
  `app/globals.css`.

## Acceptance criteria
- [x] `system.ts` defines the full Ivory Premium token set + semantic + status tokens.
- [x] `Provider.tsx` uses the custom system (not `defaultSystem`); Fraunces+Inter load; ivory canvas renders.
- [x] Existing work07 wizard inherits the tokens and still renders.
- [x] `typecheck`/`lint`/`next build` green.
- [x] Passes the **App** checklist in [../../definition-of-done.md](../../definition-of-done.md) + [frontendfeature.md §7](../../../frontendfeature.md).

## Verification
[../../verification.md](../../verification.md) → "App": `npm run typecheck && npm run lint && npm run build`;
visually confirm ivory canvas + emerald accent + Fraunces headings.

## Notes / log
- **2026-06-01 — DONE.** Shipped `linkmint-frontend/src/theme/system.ts` (ivory `#FAF7F0`, emerald
  `#0F6E4E`, champagne, status tokens; Fraunces+Inter+mono; soft shadows; focus ring + selection
  globalCss). Wired via `Provider.tsx`; fonts via Google-Fonts `<link>` in `layout.tsx` with fallback
  stacks. Build green; the work07 wizard renders under the new theme. Dark-mode seam left for work19.
