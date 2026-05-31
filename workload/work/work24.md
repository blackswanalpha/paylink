# work24 — wallet-service (read-side: balances, staking, rewards)

> **Seeded** — expand with `/work 24` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi · **Depends on:** chain RPC · **Flow:** [flow24](../flow/flow24.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.13

## Goal
A read-side surface over on-chain PLN: balances, transaction history, staking positions/rewards,
and treasury stats — indexing chain events. **Never holds private keys; returns unsigned txs.**

## In scope
- `/v1/wallets/{addr}(/transactions)`, `/v1/staking/positions`, `/staking/intent` (returns unsigned tx),
  `/staking/rewards`, `/v1/treasury/stats` (public).
- Chain indexer goroutine consuming `chain.*` (transfer/stake/unstake/reward/fee/burn) → `wallet` schema.

## Out of scope
- The build→sign→broadcast send path — that's **work34** (Phase 2): this service exposes
  read-side + unsigned intents; work34 adds `/v1/transactions(+/submit)` and client signing.
- Custody of keys or funds.

## Invariants that apply
- **Non-custodial (A.1)** — read-side only; never holds keys; staking returns **unsigned** txs.
- Settlement/balance truth from chain.

## Reuse first
- The lVM JSON-RPC (`paylink-chain/internal/rpc`) for balances/staking; the datastream + `chain.*`
  events (work15) for the indexer; staking/validator logic in `paylink-chain/internal/state`.

## Acceptance criteria
- [ ] Balance, tx history, staking positions, rewards, treasury stats served from the indexed read-side.
- [ ] `/staking/intent` returns an unsigned tx + fee estimate; no keys held.
- [ ] Indexer stays consistent with chain events; tests ≥80%.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "Full stack": stake on-chain,
confirm the read-side reflects it; request an intent and confirm it's unsigned.
