# flow12 — compliance-risk (execution recipe · seeded skeleton)

**Work item:** [work12](../work/work12.md) · **Goal recap:** basic KYC tiers + risk decision endpoint.

## Pre-flight
- [ ] Read [work12](../work/work12.md), [rules.md](../rules.md). Confirm work09 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.15 (KYC, risk model, events, KE threshold) | **Explore** |
| 2 | Design KYC session flow + risk-evaluate decision | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement KYC sessions + provider callback + risk evaluate | **service-builder** |
| 5 | Tests (tier transitions, block decisions); ≥80% | **service-builder** |
| 6 | Review invariants (PII/KMS, block path) + `/security-review` | **invariant-auditor** + `/security-review` |
| 7 | Verify KYC + a blocking decision on the stack | `/verify` |

## Done
- [x] [work12](../work/work12.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
