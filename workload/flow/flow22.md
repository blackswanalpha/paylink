# flow22 — refund-dispute-service (execution recipe · seeded skeleton)

**Work item:** [work22](../work/work22.md) · **Goal recap:** refunds + disputes/chargebacks, rail-specific reversal.

## Pre-flight
- [x] Read [work22](../work/work22.md), [rules.md](../rules.md) (A.1). work02 `done`; **work23 not
  built** — its dep is satisfied by the published `refund.clawback.requested` contract seam (repo
  precedent: work11's AuditSink before work13). A.1 non-custodial: reversal + clawback are
  instructions only.

## Steps (skeleton — refine on start)
| # | Step | Agent / Skill |
|---|------|---------------|
| 1 | Study spec §2.9 (refund/dispute lifecycles, rail reversal) | **Explore** |
| 2 | Design refund + dispute state machines + evidence + clawback | **Plan** |
| 3 | Scaffold Python/FastAPI skeleton (mirror work01) | `/scaffold-service` |
| 4 | Implement refunds + dispute intake/evidence/submit | **service-builder** |
| 5 | Wire rail reversal + clawback (settlement); tests ≥80% | **service-builder** |
| 6 | Review A.1 non-custodial + `/security-review` (money + HMAC) | **invariant-auditor** + `/security-review` |
| 7 | Verify refund + dispute paths | `/verify` |

## Done
- [x] [work22](../work/work22.md) criteria met; DoD complete; marked `done` in [backlog.md](../backlog.md) (2026-06-13).
