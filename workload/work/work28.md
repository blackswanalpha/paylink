# work28 — card adapter (Stripe)

> **Seeded** — expand with `/work 28` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go (adapters framework) · **Depends on:** 03 · **Flow:** [flow28](../flow/flow28.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.14 (adapters/card)

## Goal
A Stripe card adapter: PaymentIntent with `pl_id` in metadata, webhook signature verification,
3DS client-side, normalized to the rail-agnostic proof and broadcast to the proof-validator.

## In scope
- Stripe PaymentIntent creation + webhook verification at `/v1/adapters/card/callback`.
- Normalize to `{pl_id, rail:"card", tx_id, amount, timestamp, sender, receiver, proof_signature}`; sign; broadcast.
- Register in the orchestrator config; implement the `RailAdapter` interface.

## Out of scope
- Bank/crypto rails (work30/work29); holding funds (A.1 — buyer→merchant via Stripe).

## Invariants that apply
- **Non-custodial (A.1)**; **rail-agnostic (A.4)** — no Stripe-specific fields past the boundary; anti-replay (A.7).

## Reuse first
- The MPesa adapter pipeline (work04) as the template; `/scaffold-adapter`; signing in
  `paylink-chain/internal/crypto`; the proof-validator (work03).

## Acceptance criteria
- [ ] Verified Stripe webhook → normalized card proof → signed → broadcast → PayLink settles.
- [ ] No Stripe-specific leakage; idempotent (proof_hash UNIQUE); registered in orchestrator.
- [ ] Tests with a captured webhook; lint/build clean.
- [ ] Passes the Adapter checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Adapter" + "Full stack": replay a captured Stripe webhook,
confirm settlement via RPC.
