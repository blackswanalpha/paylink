# flow31 — subscriptions (execution recipe · seeded skeleton)

**Work item:** [work31](../work/work31.md) · **Goal recap:** recurring billing + dunning + proration on the invoice base.

## Pre-flight
- [ ] Read [work31](../work/work31.md), [rules.md](../rules.md). Confirm work19 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.6 subscription model + work19 invoice flow | **Explore** |
| 2 | Design subscription SM + scheduler + dunning + proration | **Plan** |
| 3 | Extend invoice schema + service (migration) | **service-builder** |
| 4 | Implement auto-charge + pause/resume/cancel + dunning | **service-builder** |
| 5 | Tests (cycle charge, dunning, proration); ≥80% | **service-builder** |
| 6 | Review invariants + quality | **invariant-auditor** + `/code-review` |
| 7 | Verify a cycle + a failed-charge dunning path | `/verify` |

## Done
- [ ] [work31](../work/work31.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
