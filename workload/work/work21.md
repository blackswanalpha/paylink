# work21 — fee-pricing-service (tiers, FX, quoting)

> **Done** (2026-06-03) — `linkmint-backend/fee-pricing-service/`, 76 tests @ 92%, invariants PASS (8/8), ADR-014.

- **Status:** done · **Owner:** service-builder · **Stack:** Python/FastAPI · **Depends on:** 10 · **Flow:** [flow21](../flow/flow21.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.8

## Goal
Pricing decisions: per-merchant tier, per-rail fee schedule, FX rates, and quoting; plus monthly
platform-fee invoicing. The single source of "what does this payment cost".

## In scope
- `/v1/pricing/quote`, `/v1/fx/rates`, `/v1/pricing/tiers` (admin), `/v1/merchants/{id}/pricing`, platform-fee invoices.
- Tiers (standard/growth/scale/enterprise); per-rail `pct_bps + fixed`; FX mid-market + fallback, Redis 60s cache.
- Owns `pricing` schema; consumes `merchant.onboarded`, `payment.confirmed`; publishes `pricing.*`, `fx.rate.updated`.

## Out of scope
- Volume-tier auto-upgrade + FX hedging (Phase 3).
- The on-chain protocol fee (0.5% inflation split) — that's chain-side (A.5), distinct from platform pricing.

## Invariants that apply
- Non-custodial; rates locked at quote time and stored for audit; don't conflate with the on-chain fee (A.5).

## Reuse first
- work01 Python/FastAPI layout; merchant tiers from work10; event bus (work15); ledger (work16) for fee postings.

## Acceptance criteria
- [ ] `/v1/pricing/quote` returns {gross, platform_fee, rail_fee, net, breakdown} per tier + rail.
- [ ] FX rates fetched, cached (60s), locked at quote; monthly platform-fee invoice generated.
- [ ] Tests ≥80%; lint/build clean.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Python/FastAPI)" + "Full stack".
