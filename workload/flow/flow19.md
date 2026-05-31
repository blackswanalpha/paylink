# flow19 — invoice-subscription (invoices) (execution recipe · seeded skeleton)

**Work item:** [work19](../work/work19.md) · **Goal recap:** multi-line invoices → one PayLink, paid via chain event.

## Pre-flight
- [ ] Read [work19](../work/work19.md), [rules.md](../rules.md). Confirm work01 `done`. Set `in-progress`.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.6 (invoice model, lifecycle, events) | **Explore** |
| 2 | Design schema, aggregation→PayLink, lifecycle guards | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement invoices + lines + finalize/void + PayLink aggregation | **service-builder** |
| 5 | Consume `chain.paylink.verified` → PAID; tests ≥80% | **service-builder** |
| 6 | Review invariants + quality | **invariant-auditor** + `/code-review` |
| 7 | Verify invoice → PayLink → paid | `/verify` |

## Done
- [ ] [work19](../work/work19.md) criteria met; DoD complete; mark `done` in [backlog.md](../backlog.md).
