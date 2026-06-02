# flow26 — Analytics & Reporting UI (execution recipe)

> **Seeded** — skeleton recipe; expand when backend [work26](../../work/work26.md) lands.

**Work item:** [work26](../work/work26.md) · **Goal recap:** merchant analytics (revenue/conversion/rail-mix/export) over reporting-analytics.

## Pre-flight
- [ ] Read [work26](../work/work26.md); confirm backend work26 + its SDK resource (work08) are `done`.
- [ ] Set work26 → `in-progress`.

## Steps
| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Map the reporting API + choose a charting lib | **Explore** + **Plan** | data map + decision |
| 2 | Design the analytics surface + accessible fallbacks | **Plan** | short design |
| 3 | Implement charts + date-range + export | **service-builder** | screens |
| 4 | Tests + a11y fallbacks | **service-builder** | passing |
| 5 | Review + verify | `/code-review` + `/verify` | observed |

## Done
- [ ] Acceptance criteria in [work26](../work/work26.md) met; **App** checklist complete; status `done`.
