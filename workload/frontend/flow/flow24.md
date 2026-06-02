# flow24 — Component Tests + Storybook + Visual Regression (execution recipe)

**Work item:** [work24](../work/work24.md) · **Goal recap:** Vitest component tests + a Storybook catalog + a visual-regression CI gate.

## Pre-flight
- [ ] Read [work24](../work/work24.md), [frontendfeature.md §7](../../../frontendfeature.md), the existing Vitest config + `.github/workflows/ci.yml`.
- [ ] Confirm work03 (components to test/catalog) exists.
- [ ] Set work24 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory components/hooks to cover + the CI job pattern | **Explore** | test plan |
| 2 | Design the test + Storybook + visual-regression setup | **Plan** | short design |
| 3 | Add Vitest/RTL tests + Storybook stories for `ui/*` + key hooks | **service-builder** | tests + catalog |
| 4 | Wire visual-regression + a frontend CI job | **service-builder** | green CI |
| 5 | Review coverage + flakiness | `/code-review` | clean diff |
| 6 | `npm test` + Storybook build + CI run | `/verify` | green |

## Done
- [ ] Acceptance criteria in [work24](../work/work24.md) met; **App** + Infra/CI checklists complete; closes the work08 frontend-test-job follow-up; status `done`.
