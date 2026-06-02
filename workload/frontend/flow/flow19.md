# flow19 — Theming & Dark Mode (execution recipe)

**Work item:** [work19](../work/work19.md) · **Goal recap:** dark-token parity + a persisted, SSR-safe theme toggle.

## Pre-flight
- [ ] Read [work19](../work/work19.md), [frontendfeature.md §2.2](../../../frontendfeature.md), the semantic tokens in `theme/system.ts`.
- [ ] Confirm work01 is `done`.
- [ ] Set work19 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory semantic tokens + surfaces needing dark parity | **Explore** | token + surface list |
| 2 | Design the dark palette + toggle + persistence + no-flash strategy | **Plan** | short design |
| 3 | Add `_dark` token values + the toggle + SSR-safe color-mode init | **service-builder** | dark mode |
| 4 | Audit surfaces in dark; contrast check; tests | **service-builder** | passing + AA |
| 5 | Review F.6 contrast both modes | `/code-review` | clean diff |
| 6 | Toggle + reload (no flash) across surfaces | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work19](../work/work19.md) met; **App** checklist complete; status `done`.
