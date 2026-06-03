# flow21 — fee-pricing-service (execution recipe · seeded skeleton)

**Work item:** [work21](../work/work21.md) · **Goal recap:** tiers + per-rail fees + FX + quoting + platform invoicing.

## Pre-flight
- [x] Read [work21](../work/work21.md), [rules.md](../rules.md) (A.5 distinction). Confirmed work10 `done`.

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
- [x] [work21](../work/work21.md) criteria met; DoD complete; marked `done` in [backlog.md](../backlog.md).
  - Service at `linkmint-backend/fee-pricing-service/` (port 8097, `pricing` schema). 76 tests, **92%** coverage; ruff/black/mypy clean.
  - `/v1/pricing/quote` (per tier+rail breakdown, FX locked at quote), `/v1/fx/rates`, `/v1/pricing/tiers` (admin), `/v1/pricing/merchants/{id}` + spec `/v1/merchants/{id}/pricing`; trusted `/v1/internal/accruals` + `/v1/internal/invoices/platform-fee/run` (monthly invoicing).
  - Consumes `merchant.onboarded`/`merchant.fee_tier.changed`; publishes `pricing.fee_quote.issued`/`fx.rate.updated`/`invoice.platform_fee.issued` via the outbox relay. Tolerant 5-tier superset; A.5 separation + A.6 ledger-seam OFF — see **ADR-014**.
  - Wired into docker-compose, the Kong gateway (pass-through `/v1/pricing` + `/v1/fx`), CI, and the catalog. Verified live: healthy boot, quote (USD→KES locked 129.50), accrual→invoice, outbox drained to Kafka. Invariant audit PASS (8/8).
