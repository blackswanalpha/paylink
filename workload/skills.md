# Skills — which skill to use, and when

Skills are invoked as `/<name>`. They encapsulate a repeatable capability. Use them
instead of re-deriving a procedure by hand.

## Built-in skills (relevant to LinkMint)

| Skill | Use when |
|-------|----------|
| `/code-review` | Review the current diff for correctness bugs and cleanup. Run on every work item before DoD. Use `--fix` to apply, `--comment` to post on a PR. |
| `/verify` | Run the app and observe behavior to confirm a change actually works (not just that tests pass). |
| `/run` | Launch/drive the app (or the node) to see a change live or grab a screenshot. |
| `/simplify` | Quality pass — reuse, simplification, efficiency, altitude. Does **not** hunt bugs (that's `/code-review`). |
| `/security-review` | Security review of pending changes — run before anything touching auth, proofs, signing, or money paths. |
| `/deep-research` | Multi-source, fact-checked research (e.g. Daraja API specifics, libp2p behavior) when docs in-repo aren't enough. |
| `/init` | (Re)generate a CLAUDE.md for a new sub-package — useful when scaffolding a fresh service that needs its own context file. |

## Project skills (in [`../.claude/skills/`](../.claude/skills/))

| Skill | What it does |
|-------|--------------|
| `scaffold-service` | Scaffold a new backend microservice under `linkmint-backend/` in the correct stack (**Python/FastAPI** or **Go/chi**, per ADR-003) following [`standard.md`](standard.md): directory layout, env config, structured logging, `/v1` routing + error envelope, health/readiness/metrics, idempotency hook, numbered migrations, Dockerfile, test setup, and a `docker-compose.yml` entry. |
| `scaffold-adapter` | Scaffold a new payment-rail adapter under `adapters/` implementing the receive-callback → normalize-to-proof → sign → broadcast pipeline, emitting the rail-agnostic proof shape and registering in the Payment Orchestrator config. |

## When to reach for which

- Building a brand-new service/adapter → the matching **project skill** first, then
  fill in logic with **service-builder**.
- Finished implementing → `/code-review`, then `/verify` or `/run`.
- Touching money/auth/crypto → add `/security-review`.
- Need external facts → `/deep-research`.

The full, always-current skill list is shown in the session's available-skills reminder;
this file lists the ones that matter for LinkMint work.
