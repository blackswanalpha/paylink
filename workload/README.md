# LinkMint Workload — the development operating system

This folder is the **process layer** for building LinkMint with Claude. It does not
restate the project's facts — [`../CLAUDE.md`](../CLAUDE.md) and the design docs
(`system.md`, `spec.md`, `prd.md`, `backendfeatures.md`, `blockchainfeature.md`)
remain the **source of truth**. `workload/` answers a different question:

> *Given the chain is built but the entire application layer (services, adapters, SDKs,
> apps, infra) is not — how do we build it, in order, with Claude, without losing the
> plot or breaking a protocol invariant?*

## The loop

Every unit of work follows the same cycle:

```
  pick a work item        →  work/workNN.md   (WHAT: goal, scope, done-criteria)
  read its paired flow     →  flow/flowNN.md   (HOW: ordered steps, agent/skill per step)
  execute with named agent →  .claude/agents/  +  .claude/skills/
  verify against DoD       →  definition-of-done.md  +  verification.md
  update the tracker       →  backlog.md
```

Each `workNN.md` is paired 1:1 with a `flowNN.md`. The work item is the contract;
the flow is the recipe.

## Map of this folder

| File | Purpose |
|------|---------|
| [`backlog.md`](backlog.md) | Master tracker — every work↔flow pair and its status. **Start here.** |
| [`standard.md`](standard.md) | How code is implemented (Go, TS, cross-cutting). |
| [`rules.md`](rules.md) | Protocol invariants Claude must never break + how Claude operates here. |
| [`scope.md`](scope.md) | Phase boundaries and in/out fences — the anti-scope-creep doc. |
| [`agents.md`](agents.md) | Which subagent to use, and when. |
| [`skills.md`](skills.md) | Which skill (built-in or project) to use, and when. |
| [`genie.md`](genie.md) | Prompt library — turn a vague ask into a structured prompt. |
| [`glossary.md`](glossary.md) | PayLink / lVM domain terms. |
| [`decisions.md`](decisions.md) | Lightweight ADR log. |
| [`definition-of-done.md`](definition-of-done.md) | When a work item counts as complete. |
| [`verification.md`](verification.md) | How to test/verify each change type end-to-end. |
| [`templates/`](templates/) | Templates for new work items, flows, and prompts. |
| [`work/`](work/) | Backlog items `work01.md` … `work34.md` (full `backendfeatures.md` coverage, phase-tagged). |
| [`flow/`](flow/) | Paired execution flows `flow01.md` … `flow34.md`. |
| [`frontend/`](frontend/) | The **frontend** workload subtree — work/flow pairs for the premium web UI (executes `../frontendfeature.md`). Start at [`frontend/backlog.md`](frontend/backlog.md). |

## How to drive it with Claude

The matching [`.claude/`](../.claude/) config makes this executable:

- `/work <nn>` — load `workNN.md` + `flowNN.md` and start executing the flow.
- `/new-work <title>` — scaffold the next work/flow pair from `templates/`.
- `/check-invariants` — audit the current changes against [`rules.md`](rules.md).
- Subagents: `chain-engineer` (Go/lVM), `service-builder` (TS services), `invariant-auditor` (review).
- Skills: `scaffold-service`, `scaffold-adapter`.

## Ground rules in one breath

1. Read [`rules.md`](rules.md) before touching anything — the 8 invariants are non-negotiable.
2. Stay inside the active work item's scope ([`scope.md`](scope.md)).
3. Reuse what `paylink-chain/` already gives you before writing new code.
4. Nothing ships until it passes its [`definition-of-done.md`](definition-of-done.md).
5. Update [`backlog.md`](backlog.md) when status changes.
