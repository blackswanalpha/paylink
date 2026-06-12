# flow11 — admin-backoffice (execution recipe · seeded skeleton)

**Work item:** [work11](../work/work11.md) · **Goal recap:** read-only ops console (Phase 1), audited.

## Pre-flight
- [ ] Read [work11](../work/work11.md), [rules.md](../rules.md). Confirm work09 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.4 (search, views, scopes, audit) | **Explore** |
| 2 | Design read views + scope model + audit emission | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement search + entity read views (admin JWT + MFA) | **service-builder** |
| 5 | Wire audit-log emission on every privileged access | **service-builder** |
| 6 | Tests ≥80%; review least-privilege + audit coverage | **invariant-auditor** + `/security-review` |
| 7 | Verify search + views gated correctly | `/verify` |

## Done
- [x] [work11](../work/work11.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
