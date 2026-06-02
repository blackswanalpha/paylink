# flow02 — App Shell & Navigation (execution recipe)

**Work item:** [work02](../work/work02.md) · **Goal recap:** the sidebar+topbar app frame every dashboard surface renders into.

## Pre-flight
- [ ] Read [work02](../work/work02.md), [frontendfeature.md §1](../../../frontendfeature.md).
- [ ] Confirm work01 is `done` in [../backlog.md](../backlog.md).
- [ ] Set work02 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Read the surface map + IA tree (frontendfeature.md §1) | **Explore** | nav model |
| 2 | Design the shell layout + nav model (live flags) | **Plan** | short design |
| 3 | Implement `AppShell`/`Sidebar`/`Topbar`/`nav.ts` on the theme tokens | **service-builder** | the shell |
| 4 | Wire a real surface (the dashboard, work18) into the shell | **service-builder** | shell in use |
| 5 | typecheck/lint/build; keyboard + responsive spot-check | `/verify` | green + screenshot |

## Done
- [x] Acceptance criteria in [work02](../work/work02.md) met.
- [x] **App** checklist complete; status `done`; mobile-drawer → work20, command palette → work23 filed.
