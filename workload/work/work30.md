# work30 — bank adapter (Plaid / GoCardless / TrueLayer)

> **Seeded** — expand with `/work 30` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go (adapters framework) · **Depends on:** 03 · **Flow:** [flow30](../flow/flow30.md)
- **Phase:** 3 / Mainnet · **Spec:** backendfeatures.md §2.14 (adapters/bank)

## Goal
A bank-transfer adapter (region-specific: Plaid US, GoCardless/TrueLayer EU/UK) modeling T+1
settlement, normalized to the rail-agnostic proof and broadcast to the proof-validator.

## In scope
- Region provider integration at `/v1/adapters/bank/callback`; T+1 settlement modeling.
- Normalize to `{..., rail:"bank", ...}`; sign; broadcast; register in orchestrator.

## Out of scope
- Other rails; custody (A.1 — sender→receiver via the bank rail).

## Invariants that apply
- **Non-custodial (A.1)**; **rail-agnostic (A.4)**; anti-replay (A.7).

## Reuse first
- The MPesa/card adapter pipeline (work04/work28) template; `/scaffold-adapter`; proof-validator (work03).

## Acceptance criteria
- [ ] A provider callback normalizes to a bank proof → signed → broadcast → settlement (T+1 modeled).
- [ ] No provider-specific leakage; idempotent; registered in orchestrator.
- [ ] Tests with a captured callback; lint/build clean.
- [ ] Passes the Adapter checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Adapter" + "Full stack".
