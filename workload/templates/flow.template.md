# flowNN — <Title> (execution recipe)

> The **recipe** for [workNN](../work/workNN.md). Ordered steps; each step names the
> agent/skill to use. Copy this file to `flow/flowNN.md`.

**Work item:** [workNN](../work/workNN.md) · **Goal recap:** <one line>

## Pre-flight
- [ ] Read [workNN](../work/workNN.md), [rules.md](../rules.md), and the item's scope fence.
- [ ] Confirm dependencies are `done` in [backlog.md](../backlog.md).
- [ ] Set the item to `in-progress` in [backlog.md](../backlog.md).

## Steps

| # | Step | Agent / Skill | Output |
|---|------|---------------|--------|
| 1 | Understand the relevant existing code | **Explore** | map of what to reuse |
| 2 | Design the approach | **Plan** | short design |
| 3 | Implement | **chain-engineer** / **service-builder** / `scaffold-*` | code |
| 4 | Test | (same as 3) | passing tests, coverage |
| 5 | Review against invariants | **invariant-auditor** + `/code-review` | clean diff |
| 6 | Verify end-to-end | `/verify` or `/run` | observed working |

<Replace the rows above with the concrete steps for this item.>

## Done
- [ ] Acceptance criteria in [workNN](../work/workNN.md) all met.
- [ ] [definition-of-done.md](../definition-of-done.md) checklist for the change type complete.
- [ ] Status set to `done` in [backlog.md](../backlog.md); follow-ups filed.
