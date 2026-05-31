---
description: Load a work item + its paired flow and start executing it
argument-hint: <nn>  (e.g. 01)
---

You are starting work item **$1** from the LinkMint workload system.

1. Read `workload/work/work$1.md` (the contract) and `workload/flow/flow$1.md` (the recipe).
   If either file is missing, tell the user and stop.
2. Read `workload/rules.md` (invariants + how-we-work) and the work item's scope fence
   (`workload/scope.md`). Read `workload/standard.md` for the relevant language.
3. Check `workload/backlog.md`: confirm this item's dependencies are `done`. If a dependency
   is not done, warn the user and ask whether to proceed anyway.
4. If the flow is a seeded skeleton (items 03–08), first expand it into concrete steps for this
   item (still within scope), then proceed.
5. Walk the flow's steps in order, using the named agent/skill for each step:
   - **Explore** for "understand", **Plan** for "design",
   - **chain-engineer** (Go) or **service-builder** (TS) / `/scaffold-service` / `/scaffold-adapter`
     for "implement",
   - **invariant-auditor** + `/code-review` for "review", `/verify` or `/run` for "verify".
6. Before declaring done, run the matching checklist in `workload/definition-of-done.md` and the
   relevant section of `workload/verification.md`. Report commands run and their pass/fail output.
7. Update `workload/backlog.md`: set status to `in-progress` when you start and `done` when the
   acceptance criteria + DoD are met. File any discovered side-work as a new backlog row — do not
   expand the current item.

Stay within the work item's scope. If a step seems to require breaking an invariant, stop and
surface it (it likely needs an ADR in `workload/decisions.md`).
