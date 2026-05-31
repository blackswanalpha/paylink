# flow09 — identity-service (execution recipe · seeded skeleton)

**Work item:** [work09](../work/work09.md) · **Goal recap:** auth + users/orgs/API keys + JWT/OAuth/MFA.

## Pre-flight
- [ ] Read [work09](../work/work09.md), [rules.md](../rules.md). Confirm work15/16/17 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.2 (endpoints, roles, data model, events) | **Explore** |
| 2 | Design auth flows (JWT/OAuth/MFA), schema, RBAC | **Plan** |
| 3 | Scaffold the Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement auth + users/orgs/API keys/sessions | **service-builder** |
| 5 | Tests (auth round-trips, RBAC, MFA); ≥80% | **service-builder** |
| 6 | Review invariants (secrets, non-custodial) + `/security-review` | **invariant-auditor** + `/security-review` |
| 7 | Verify register→login→authed call on the stack | `/verify` |

## Done
- [x] [work09](../work/work09.md) criteria met; DoD complete; marked `done` in [backlog.md](../backlog.md).
