# flow16 — double-entry ledger (execution recipe · seeded skeleton)

**Work item:** [work16](../work/work16.md) · **Goal recap:** append-only double-entry ledger + posting helpers.

## Pre-flight
- [ ] Read [work16](../work/work16.md), [rules.md](../rules.md) (A.6). Confirm work15 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study the ledger schema + fee split in `internal/fee` | **Explore** |
| 2 | Design posting helper API + balance invariant + correction model | **Plan** |
| 3 | Create `ledger` schema + numbered migration | **service-builder** |
| 4 | Implement Python + Go posting helpers (atomic, balanced) | **service-builder** |
| 5 | Tests (balanced/unbalanced/append-only/reversal); ≥80% | **service-builder** |
| 6 | Review A.6 (append-only, balance) rigorously | **invariant-auditor** + `/code-review` |
| 7 | Verify posting + rejection + correction | `/verify` |

## Done
- [ ] [work16](../work/work16.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
