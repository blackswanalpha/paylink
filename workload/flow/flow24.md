# flow24 — wallet-service (execution recipe · seeded skeleton)

**Work item:** [work24](../work/work24.md) · **Goal recap:** read-side over on-chain PLN; unsigned-tx intents; no custody.

## Pre-flight
- [ ] Read [work24](../work/work24.md), [rules.md](../rules.md) (A.1). Confirm chain RPC reachable. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Map lVM RPC balance/staking methods + `chain.*` events | **Explore** |
| 2 | Design indexer + read APIs + unsigned-intent flow | **Plan** |
| 3 | Scaffold Go/chi skeleton (mirror work02) | `/scaffold-service` |
| 4 | Implement indexer goroutine + read endpoints | **service-builder** |
| 5 | Implement `/staking/intent` (unsigned tx); tests ≥80% | **service-builder** |
| 6 | Review A.1 (no keys, unsigned only) rigorously | **invariant-auditor** + `/code-review` |
| 7 | Verify read-side reflects an on-chain stake | `/verify` |

## Done
- [ ] [work24](../work/work24.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
