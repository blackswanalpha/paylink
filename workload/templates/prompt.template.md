# Prompt template — the Genie structure

> Fill these six fields before kicking off a non-trivial task. See worked examples in
> [genie.md](../genie.md). Keep each field tight; link to repo docs instead of restating them.

```
Context:      <work item + current state of the code + design doc reference>
Task:         <one imperative sentence — the outcome>
Constraints:  <which rules.md invariants apply; what scope.md says is OUT of scope>
Reuse:        <existing functions/types/packages/services to build on, with paths>
Acceptance:   - <testable bullet>
              - <testable bullet>
Verify:       <exact commands / steps from verification.md>
```

**Readiness check:** if you can't fill in *Acceptance* and *Verify*, the task isn't ready —
clarify scope or split it in [backlog.md](../backlog.md) first.
