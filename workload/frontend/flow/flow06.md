# flow06 — Loading, Empty & Skeleton States (execution recipe)

**Work item:** [work06](../work/work06.md) · **Goal recap:** a system for loading/empty/optimistic states so no screen shows a bare spinner or blank panel.

## Pre-flight
- [ ] Read [work06](../work/work06.md), [frontendfeature.md §1](../../../frontendfeature.md), the `Skeleton`/`EmptyState` primitives + the dashboard `initializing` pattern.
- [ ] Confirm work03 is usable.
- [ ] Set work06 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Catalog the surfaces + their loading/empty shapes | **Explore** | per-surface state list |
| 2 | Design skeleton compositions + `Loadable`/`AsyncBoundary` + optimistic helper + empty catalog | **Plan** | short design |
| 3 | Implement compositions, the boundary (Suspense + work04 error seam), optimistic helper, empty catalog | **service-builder** | the system |
| 4 | Apply to ≥1 real surface (cancel = optimistic) + tests | **service-builder** | used + passing |
| 5 | Review a11y (`aria-busy`) + consistency | `/code-review` | clean diff |
| 6 | Throttle network + empty creator + cancel flow | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work06](../work/work06.md) met; **App** checklist complete; status `done`.
