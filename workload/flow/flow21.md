# flow21 — fee-pricing-service (execution recipe · seeded skeleton)

**Work item:** [work21](../work/work21.md) · **Goal recap:** tiers + per-rail fees + FX + quoting + platform invoicing.

## Pre-flight
- [ ] Read [work21](../work/work21.md), [rules.md](../rules.md) (A.5 distinction). Confirm work10 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.8 (pricing model, FX, invoicing) | **Explore** |
| 2 | Design tier/rail schedule + FX cache + quote breakdown | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement quoting + FX + tiers + platform invoices | **service-builder** |
| 5 | Tests (quote math, FX lock, invoice gen); ≥80% | **service-builder** |
| 6 | Review A.5 separation + quality | **invariant-auditor** + `/code-review` |
| 7 | Verify a quote breakdown end-to-end | `/verify` |

## Done
- [ ] [work21](../work/work21.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
