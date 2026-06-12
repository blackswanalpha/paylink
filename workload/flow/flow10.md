# flow10 — merchant-onboarding (execution recipe · seeded skeleton)

**Work item:** [work10](../work/work10.md) · **Goal recap:** merchant verification, bank linking, contracts, fee-tier.

## Pre-flight
- [ ] Read [work10](../work/work10.md), [rules.md](../rules.md). Confirm work09 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.3 (endpoints, state machine, data model) | **Explore** |
| 2 | Design schema, state machine, document/bank handling | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement onboarding + docs (S3) + bank linking + contracts | **service-builder** |
| 5 | Tests for the state machine + verification paths; ≥80% | **service-builder** |
| 6 | Review invariants (KMS encryption, non-custodial) | **invariant-auditor** + `/security-review` |
| 7 | Verify onboarding → ACTIVE on the stack | `/verify` |

## Done
- [x] [work10](../work/work10.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
