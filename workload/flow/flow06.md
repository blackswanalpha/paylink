# flow06 — JS/TS SDK (execution recipe · seeded skeleton)

**Work item:** [work06](../work/work06.md) · **Goal recap:** typed `/v1` client for PayLinks +
payments, error-envelope aware.

## Pre-flight
- [ ] Read [work06](../work/work06.md), [standard.md](../standard.md). Confirm work05 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)

| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Inventory the `/v1` endpoints + request/response + error envelope | **Explore** (services + docs/api) |
| 2 | Design the client surface (types, auth, error mapping) | **Plan** |
| 3 | Implement typed client (strict, no `any`) | **service-builder** |
| 4 | Implement auth pass-through + typed error mapping | **service-builder** |
| 5 | Unit tests vs mock server (success + error paths) | **service-builder** |
| 6 | Review vs A.4 + TS standards | **invariant-auditor** + `/code-review` |
| 7 | Verify against local stack (create→read→settle) | `/verify` |

## Done
- [ ] [work06](../work/work06.md) criteria met; SDK DoD complete; mark `done` in [backlog.md](../backlog.md).
