# flow11 — PayLinks Management (execution recipe)

**Work item:** [work11](../work/work11.md) · **Goal recap:** list + create-modal + detail-drawer + cancel + share/QR over the live paylinks API.

## Pre-flight
- [ ] Read [work11](../work/work11.md), [frontendfeature.md §3.3](../../../frontendfeature.md), backend [work01](../../work/work01.md) (paylink endpoints), the work07 `CreatePayLinkForm`.
- [ ] Confirm work03, work04 are usable (Modal/Drawer/DataTable + error system).
- [ ] Set work11 → `in-progress`.

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map `paylinks.list/get/create/cancel` + the work07 create form to reuse | **Explore** | reuse map |
| 2 | Design list (filter/cursor), create-modal (from the form), detail-drawer, cancel/share | **Plan** | short design |
| 3 | Implement the list + Create modal (refactor CreatePayLinkForm) + detail Drawer + cancel + share/QR | **service-builder** | the workspace |
| 4 | Optimistic cancel + 402 KYC handling (work04) + tests | **service-builder** | passing |
| 5 | Review F.2/F.3 + a11y | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Create→list→drawer→cancel + 402 path against the live stack | `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work11](../work/work11.md) met; **App** checklist complete; create-modal noted as the pattern for work14; status `done`.
