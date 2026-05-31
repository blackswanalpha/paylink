# flow28 — card adapter (Stripe) (execution recipe · seeded skeleton)

**Work item:** [work28](../work/work28.md) · **Goal recap:** Stripe webhook → card proof → sign → broadcast.

## Pre-flight
- [ ] Read [work28](../work/work28.md), [rules.md](../rules.md) (A.1/A.4). Confirm work03 `done`. Stripe sandbox keys in env (never commit). Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Confirm Stripe PaymentIntent + webhook shape; map to proof | **Explore** + `/deep-research` |
| 2 | Design adapter (mirror work04 pipeline) | **Plan** |
| 3 | Scaffold the adapter | `/scaffold-adapter` |
| 4 | Implement PaymentIntent + webhook verify + normalize + sign + broadcast | **service-builder** |
| 5 | Register in orchestrator; tests with captured webhook | **service-builder** |
| 6 | Review A.1/A.4 + `/security-review` (money + secrets) | **invariant-auditor** + `/security-review` |
| 7 | Verify end-to-end settlement | `/verify` |

## Done
- [ ] [work28](../work/work28.md) criteria met; Adapter DoD complete; mark `done` in [backlog.md](../backlog.md).
