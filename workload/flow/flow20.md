# flow20 — escrow-manager (execution recipe · seeded skeleton)

**Work item:** [work20](../work/work20.md) · **Goal recap:** conditional release/refund without custody.

## Pre-flight
- [ ] Read [work20](../work/work20.md), [rules.md](../rules.md) (A.1!). Confirm work01 + work03 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.7 (conditions, state machine) + PayLink FSM | **Explore** |
| 2 | Design condition evaluation + non-custodial release/refund | **Plan** |
| 3 | Scaffold Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement escrow lifecycle + 3 condition types | **service-builder** |
| 5 | Consume `chain.paylink.verified`; tests ≥80% | **service-builder** |
| 6 | Review **A.1 non-custodial** rigorously + quality | **invariant-auditor** + `/code-review` |
| 7 | Verify release + timeout-refund paths | `/verify` |

## Done
- [ ] [work20](../work/work20.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
