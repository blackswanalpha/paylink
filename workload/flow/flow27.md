# flow27 — reconciliation-service (execution recipe · seeded skeleton)

**Work item:** [work27](../work/work27.md) · **Goal recap:** daily 3-way recon DB↔chain↔rails, classify discrepancies.

## Pre-flight
- [ ] Read [work27](../work/work27.md), [rules.md](../rules.md). Confirm work23 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.20 (recon algorithm, discrepancy classes) | **Explore** |
| 2 | Design the 3-way join + classification + alerting | **Plan** |
| 3 | Scaffold Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement run + join + classify + resolve | **service-builder** |
| 5 | Tests with injected mismatches per class; ≥80% | **service-builder** |
| 6 | Review A.7 authority + read-only sources | **invariant-auditor** + `/code-review` |
| 7 | Verify detection + classification on a seeded day | `/verify` |

## Done
- [ ] [work27](../work/work27.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
