# flow29 — crypto adapter (execution recipe · seeded skeleton)

**Work item:** [work29](../work/work29.md) · **Goal recap:** per-PayLink address → watch → N-confirm → proof → broadcast.

## Pre-flight
- [ ] Read [work29](../work/work29.md), [rules.md](../rules.md) (A.1/A.4/A.7). Confirm work03 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Confirm deterministic-address + confirmation model; map to proof | **Explore** + `/deep-research` |
| 2 | Design watcher + N-confirmation → proof pipeline | **Plan** |
| 3 | Scaffold the adapter | `/scaffold-adapter` |
| 4 | Implement address derivation + watcher + normalize + sign + broadcast | **service-builder** |
| 5 | Register in orchestrator; tests with simulated confirmation | **service-builder** |
| 6 | Review A.1/A.4/A.7 + `/security-review` | **invariant-auditor** + `/security-review` |
| 7 | Verify end-to-end settlement | `/verify` |

## Done
- [ ] [work29](../work/work29.md) criteria met; Adapter DoD complete; mark `done` in [backlog.md](../backlog.md).
