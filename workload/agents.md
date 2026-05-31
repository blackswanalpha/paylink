# Agents — which subagent to use, and when

Subagents do focused work in their own context and return a result. Pick the smallest
agent that fits; don't spawn a fleet for a one-file lookup.

## Built-in agents

| Agent | Use when | Don't use for |
|-------|----------|---------------|
| **Explore** | Read-only search across many files/dirs — "where is X", "map the Y package", naming-convention sweeps. Returns conclusions, not file dumps. | Editing, or single-file reads you already know the path to. |
| **Plan** | Designing an implementation approach before building a non-trivial work item. | Trivial edits; pure research. |
| **general-purpose** | Multi-step tasks that mix searching and acting and don't fit a specialized agent. | When Explore (read-only) or a project agent is a tighter fit. |

## Project subagents (in [`../.claude/agents/`](../.claude/agents/))

| Agent | Domain | Encodes |
|-------|--------|---------|
| **chain-engineer** | Go / lVM work in `paylink-chain/` — new tx types, executor cases, consensus, fee, rules, tests. | [`standard.md`](standard.md) Go section + the tx-type recipe + [`rules.md`](rules.md) invariants 2–8. |
| **service-builder** | Backend services (**Python/FastAPI** + **Go/chi**, per ADR-003) and the TS SDK/web — scaffolding and feature work under `linkmint-backend/`, `adapters/`, `sdks/`, `apps/`. Picks stack from the work item. | [`standard.md`](standard.md) backend section (12-factor, `/v1` + error envelope, idempotency, migrations, 80% coverage) + per-stack rules. |
| **invariant-auditor** | Reviewing a diff against the protocol invariants before it's considered done. | [`rules.md`](rules.md) Part A — flags custody, EVM/contract patterns, replay holes, rail leakage, fee-split changes. |

## Pairing agents with the loop

- **Explore** during a flow's "understand" step.
- **Plan** during a flow's "design" step (or the `Plan` phase of a work item).
- **chain-engineer** (lVM/Go) or **service-builder** (Python/FastAPI, Go/chi, or TS) during
  "implement" steps, chosen by the work item's stack.
- **invariant-auditor** + the `code-review` skill during the "verify" step.

See [`skills.md`](skills.md) for skills (`/code-review`, `/verify`, `/run`, etc.), and
[`flow/`](flow/) for which agent each step of a given work item should use.
