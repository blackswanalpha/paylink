---
description: Scaffold the next work item + paired flow from the workload templates
argument-hint: <title of the new work item>
---

Create the next work/flow pair in the LinkMint workload system for: **$ARGUMENTS**

1. Determine the next number `NN` = (highest existing `workload/work/workNN.md`) + 1, zero-padded
   to two digits.
2. Copy `workload/templates/work.template.md` → `workload/work/work<NN>.md` and
   `workload/templates/flow.template.md` → `workload/flow/flow<NN>.md`. Replace every `NN` and
   `<Title>` placeholder with the real number and the title from `$ARGUMENTS`.
3. Fill in the work item using the **Genie structure** (`workload/genie.md`): Goal, Why/context
   (link the relevant design doc section), In scope, Out of scope, Invariants that apply (cite
   `workload/rules.md` items), Reuse-first pointers (real paths), Acceptance criteria (testable),
   Verification (commands from `workload/verification.md`). Ask the user for anything you can't
   infer — do not guess scope.
4. Fill the flow with concrete ordered steps, naming the agent/skill for each (see
   `workload/agents.md` and `workload/skills.md`).
5. Add a row to the table in `workload/backlog.md` with status `todo`, the dependencies, and links
   to the new work/flow files. Add a dated line to its changelog.
6. Confirm to the user the new files and the backlog row, and remind them they can start it with
   `/work <NN>`.

Respect `workload/scope.md`: if the requested item is out of the current phase, create it but mark
it clearly as deferred in its scope section.
