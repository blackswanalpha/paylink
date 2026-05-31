# flow32 — SDK suite (execution recipe · seeded skeleton)

**Work item:** [work32](../work/work32.md) · **Goal recap:** Python/Go/Java/Flutter SDKs in parity with the JS SDK.

## Pre-flight
- [ ] Read [work32](../work/work32.md), [standard.md](../standard.md). Confirm work06 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Review the JS SDK surface (work06) + OpenAPI spec | **Explore** |
| 2 | Design the shared client contract; per-language idioms | **Plan** |
| 3–6 | For each language: implement typed client + error mapping + tests | **service-builder** (one pass per language) |
| 7 | Review parity vs JS SDK; rail-agnostic check | **invariant-auditor** + `/code-review` |
| 8 | Verify each SDK against the local stack | `/verify` |

## Done
- [ ] [work32](../work/work32.md) criteria met; SDK DoD complete per language; mark `done` in [backlog.md](../backlog.md).
