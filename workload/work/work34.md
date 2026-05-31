# work34 — Token send & payment submission (build → sign → broadcast)

> **Seeded** — expand with `/work 34` when picked up.

- **Status:** todo · **Owner:** service-builder · **Stack:** Go/chi (wallet-service) + TypeScript (SDK) · **Depends on:** 24, 06 · **Flow:** [flow34](../flow/flow34.md)
- **Phase:** 2 / Beta · **Spec:** backendfeatures.md §2.13 (wallet, "submit … or relay") + lVM `TxTransfer` / `paylink_sendTransaction`

## Goal
The **payer-side send path**: let a user construct, sign, and broadcast a token-transfer (and a
pay-a-PayLink) transaction so tokens actually move to make a payment. Closes the gap where the
backlog only *receives* (work29) and *reads* (work24) — nothing *sent*.

## Why / context
The chain already supports the primitive (`TxTransfer` in `paylink-chain/internal/chain/executor.go`,
broadcast via `paylink_sendTransaction`), and wallet-service (work24) returns *unsigned* txs but
defers submission to Phase 3. This item provides the end-to-end **build → sign → broadcast** flow,
**non-custodially** — keys never leave the client.

## In scope
- **wallet-service (Go/chi):** `POST /v1/transactions` builds an **unsigned** tx (transfer, or
  pay-PayLink) with the correct nonce (`paylink_getNonce`) and fee estimate; `POST
  /v1/transactions/submit` accepts an **already-signed** tx and broadcasts it via
  `paylink_sendTransaction` (thin relay — never sees a private key).
- **JS SDK (TS):** client-side ECDSA signing that reproduces the chain's transaction
  `SignableBytes()` byte-for-byte; helpers `buildTransfer/buildPayment → sign(localKey) → submit`.
- Status read-back: the sent transfer reflected via wallet-service/RPC; receipt via `paylink_getTransactionReceipt`.

## Out of scope (do NOT do here)
- Server-side key storage or custodial signing of any kind (A.1).
- New chain tx types (use the existing `TxTransfer`; if paying a PayLink needs a dedicated tx,
  that's a chain item, not this app item).
- Other-language SDK signing (work32, Phase 3).

## Invariants that apply
- **Non-custodial (A.1)** — keys are client-side only; the relay broadcasts pre-signed bytes and
  must reject any request containing key material.
- **Anti-replay (A.7)** — correct per-sender nonce; the chain's proof-hash / nonce checks remain authoritative.
- **No EVM (A.2)** — native `TxTransfer`, not a contract call.

## Reuse first
- `paylink-chain/internal/types/transaction.go` (`SignableBytes()`) and `internal/crypto/signing.go`
  (ECDSA over secp256k1) — the SDK signer must match these byte-exactly (mirror how work03 imports them).
- wallet-service (work24) build/unsigned-intent code; `paylink_getNonce`, `paylink_sendTransaction`,
  `paylink_getTransactionReceipt` RPC methods; the JS SDK client (work06).

## Acceptance criteria
- [ ] `POST /v1/transactions` returns an unsigned tx with correct nonce + fee estimate.
- [ ] SDK signs locally and produces bytes the chain accepts (round-trips against `SignableBytes`).
- [ ] `POST /v1/transactions/submit` broadcasts the signed tx; sender balance/receipt update on-chain.
- [ ] The relay holds/accepts **no private keys**; a request carrying key material is rejected.
- [ ] Tests (build, sign-verify parity, submit, replay-nonce) ≥80%; lint/build clean.
- [ ] Passes the Backend-service + SDK checklists in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Backend service (Go/chi)" + "SDK" + "Full stack":
build → sign in the SDK → submit → confirm the transfer on-chain via receipt + balance; attempt a
re-submit with a stale nonce and confirm rejection.

## Notes / log
- The hardest part is **byte-exact serialization parity** between the Go `SignableBytes()` and the
  TS signer — pin it with a shared test vector (sign in Go, verify in TS, and vice-versa).
