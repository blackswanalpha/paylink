# flow22 — Accessibility Audit & Hardening (execution recipe)

**Work item:** [work22](../work/work22.md) · **Goal recap:** a WCAG 2.1 AA pass + an axe CI gate across the app.

## Pre-flight
- [ ] Read [work22](../work/work22.md), [frontendfeature.md §2.7](../../../frontendfeature.md) + F.6.
- [ ] Confirm the built surfaces (03–18) exist.
- [ ] Set work22 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Audit each surface (keyboard/focus/aria/contrast/landmarks) + run axe to baseline | **Explore** / `/run` | findings list |
| 2 | Prioritize + design fixes (focus trap/restore, live regions, tab order) | **Plan** | fix plan |
| 3 | Apply fixes across components/surfaces | **service-builder** | a11y fixes |
| 4 | Add jest-axe to component tests + an axe pass in CI | **service-builder** | green axe gate |
| 5 | Review F.6 coverage | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Keyboard-only walkthrough + axe per route | `/verify` | 0 serious/critical |

## Done
- [ ] Acceptance criteria in [work22](../work/work22.md) met; **App** checklist complete; axe gate in CI; status `done`.
