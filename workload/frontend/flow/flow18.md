# flow18 — Merchant Dashboard Overview (execution recipe)

**Work item:** [work18](../work/work18.md) · **Goal recap:** the flagship `/dashboard` overview — metrics + sparkline + recent PayLinks on live data.

## Pre-flight
- [ ] Read [work18](../work/work18.md), [frontendfeature.md §3.3](../../../frontendfeature.md), backend [work01](../../work/work01.md).
- [ ] Confirm work03 + the shell (work02) are usable.
- [ ] Set work18 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `paylinks.list` + the aggregate shape | **Explore** | data map |
| 2 | Design the overview layout + `usePayLinks` aggregation | **Plan** | short design |
| 3 | Implement `MerchantDashboard` + `usePayLinks` + the server page (mint JWT) | **service-builder** | the screen |
| 4 | Skeletons/empty/error states; tests | **service-builder** | passing |
| 5 | Review F.7 (analytics PLANNED, not faked) | `/code-review` | clean diff |
| 6 | Seed PayLinks; open `/dashboard` against live stack | `/verify` | observed |

## Done
- [x] Acceptance criteria in [work18](../work/work18.md) met.
- [x] **App** checklist complete; richer analytics filed as work26; status `done`.
