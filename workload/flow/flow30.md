# flow30 — bank adapter (execution recipe · seeded skeleton)

**Work item:** [work30](../work/work30.md) · **Goal recap:** bank provider callback → bank proof → broadcast (T+1).

## Pre-flight
- [ ] Read [work30](../work/work30.md), [rules.md](../rules.md) (A.1/A.4/A.7). Confirm work03 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Confirm provider callback shape + T+1 model; map to proof | **Explore** + `/deep-research` |
| 2 | Design adapter (mirror work04/28) | **Plan** |
| 3 | Scaffold the adapter | `/scaffold-adapter` |
| 4 | Implement provider integration + normalize + sign + broadcast | **service-builder** |
| 5 | Register in orchestrator; tests with captured callback | **service-builder** |
| 6 | Review A.1/A.4/A.7 + `/security-review` | **invariant-auditor** + `/security-review` |
| 7 | Verify settlement (T+1 modeled) | `/verify` |

## Done
- [ ] [work30](../work/work30.md) criteria met; Adapter DoD complete; mark `done` in [backlog.md](../backlog.md).
