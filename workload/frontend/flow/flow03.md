# flow03 — Core Component Library (execution recipe)

**Work item:** [work03](../work/work03.md) · **Goal recap:** the token-driven enterprise component kit feature pages compose from.

## Pre-flight
- [ ] Read [work03](../work/work03.md), [frontendfeature.md §2.5](../../../frontendfeature.md).
- [ ] Confirm work01, work02 are `done`.
- [ ] Set work03 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Inventory shipped primitives vs the §2.5 spec; list the gap | **Explore** (`src/components/ui`) | gap list (Modal/Drawer/Tabs/Form/DataTable/…) |
| 2 | Design wrappers over Chakra v3 compounds with token recipes + a11y | **Plan** | component API sketch |
| 3 | Implement the interactive components (Modal/Drawer/Tabs/FormField/DataTable/CopyField/QRBlock/Stepper) | **service-builder** | the kit |
| 4 | Add a kitchen-sink render + smoke tests | **service-builder** | renders + passes |
| 5 | Review a11y + token usage | **invariant-auditor** + `/code-review` | clean diff |
| 6 | typecheck/lint/build; keyboard pass | `/verify` | green |

## Done
- [ ] Acceptance criteria in [work03](../work/work03.md) met.
- [ ] **App** checklist complete; Storybook/visual-regression filed as work24; status `done`.
