# flow01 — Design System & Theme (execution recipe)

> The **recipe** for [work01](../work/work01.md).

**Work item:** [work01](../work/work01.md) · **Goal recap:** the Ivory Premium tokens as a Chakra v3 system — the root dependency.

## Pre-flight
- [ ] Read [work01](../work/work01.md), the frontend invariants in [frontendfeature.md](../../../frontendfeature.md), and [../../standard.md](../../standard.md) "TypeScript".
- [ ] No dependencies — clear to start.
- [ ] Set work01 → `in-progress` in [../backlog.md](../backlog.md).

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Read frontendfeature.md §2 (tokens, type scale, elevation, motion) | **Explore** | the token spec to encode |
| 2 | Design the token map + semantic/status tokens + font-load strategy | **Plan** | short design |
| 3 | Implement `theme/system.ts` (`createSystem`) + wire `Provider.tsx`, `layout.tsx` fonts, `globals.css` | **service-builder** | the live theme |
| 4 | Confirm the work07 wizard inherits tokens (no regression) | **service-builder** | wizard renders under new theme |
| 5 | typecheck/lint/build; visual spot-check | `/verify` | green gates + screenshot |

## Done
- [x] Acceptance criteria in [work01](../work/work01.md) met.
- [x] **App** checklist in [../../definition-of-done.md](../../definition-of-done.md) complete.
- [x] Status `done` in [../backlog.md](../backlog.md); dark-mode follow-up filed as work19.
