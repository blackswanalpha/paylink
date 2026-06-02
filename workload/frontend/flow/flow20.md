# flow20 — Responsive & Mobile (execution recipe)

**Work item:** [work20](../work/work20.md) · **Goal recap:** mobile drawer + table→card collapse + touch sizing across the app.

## Pre-flight
- [ ] Read [work20](../work/work20.md), [frontendfeature.md §2.7](../../../frontendfeature.md), the shell + DataTable responsive seams.
- [ ] Confirm work02, work03 are usable.
- [ ] Set work20 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Audit surfaces at 360/768/1280px; list breakpoints | **Explore** | responsive gap list |
| 2 | Design the drawer, table→card, sheet-modal, breakpoint scale | **Plan** | short design |
| 3 | Implement the mobile drawer + responsive DataTable + sheet modals + reflow | **service-builder** | responsive app |
| 4 | Touch sizing + safe areas; tests | **service-builder** | passing |
| 5 | Review F.6 on collapsed layouts | `/code-review` | clean diff |
| 6 | Exercise at multiple viewports | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work20](../work/work20.md) met; **App** checklist complete; status `done`.
