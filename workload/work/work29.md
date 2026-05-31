# work29 — crypto adapter (on-chain stablecoin)

> **Seeded** — expand with `/work 29` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go (adapters framework) · **Depends on:** 03 · **Flow:** [flow29](../flow/flow29.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.14 (adapters/crypto)

## Goal
A crypto adapter: a deterministic receive address per PayLink, a chain-watcher/subscription, and
on N-confirmation a normalized proof signed + broadcast to the proof-validator.

## In scope
- Deterministic deposit address per PayLink; watch for inbound stablecoin transfer.
- On N confirmations → normalize to `{..., rail:"crypto", ...}`; sign; broadcast at `/v1/adapters/crypto/callback`.
- Register in the orchestrator config; implement the `RailAdapter` interface.

## Out of scope
- Stellar/Solana variants (later); bank/card rails; custody of funds (A.1 — sender→receiver on-chain).

## Invariants that apply
- **Non-custodial (A.1)**; **rail-agnostic (A.4)**; anti-replay (A.7) — one confirmed transfer settles one PayLink.

## Reuse first
- The MPesa adapter pipeline (work04) template; `/scaffold-adapter`; crypto in `paylink-chain/internal/crypto`;
  the proof-validator (work03).

## Acceptance criteria
- [ ] Inbound transfer to the per-PayLink address detected at N confirmations → proof → settlement.
- [ ] No crypto-specific leakage past the boundary; idempotent; registered in orchestrator.
- [ ] Tests with a simulated confirmation; lint/build clean.
- [ ] Passes the Adapter checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Adapter" + "Full stack": simulate a confirmed transfer,
confirm settlement via RPC.
