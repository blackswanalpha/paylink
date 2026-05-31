# work03 — proof-validator (verify proof + broadcast to lVM RPC)

> **Seeded** — expand with `/work 03` when picked up. Goal, fences, and criteria are set;
> flesh out the implementation detail at start.

- **Status:** done · **Owner:** service-builder · **Stack:** Go/chi (ADR-003) · **Depends on:** 02 · **Flow:** [flow03](../flow/flow03.md)
- **Phase:** MVP (see [scope.md](../scope.md))

## Goal
Build `linkmint-backend/proof-validator` — the off-chain bridge that verifies a normalized payment
proof's signature/shape and broadcasts a settlement transaction to the lVM via JSON-RPC.

## Why / context
Adapters produce signed proofs; the chain settles them. The proof-validator is the trusted
verification + broadcast hop in between (`../../system.md` "Proof Validator"). In the MVP's
single-validator model it is the path from "rail says paid" to "chain settles".

## In scope
- Verify the rail-agnostic proof shape `{pl_id, rail, tx_id, amount, timestamp, sender,
  receiver, proof_signature}` and its signature.
- Broadcast the corresponding settlement tx to the lVM JSON-RPC.
- Surface accept/reject with the standard error envelope; structured logging.
- Tests ≥80%; Dockerfile + docker-compose entry.

## Out of scope
- Rail-specific parsing → work04 (adapter normalizes first).
- Multi-validator committee/quorum logic → that lives **on-chain** already (`internal/consensus`);
  don't reimplement consensus off-chain.
- Orchestration/lifecycle → work02.

## Invariants that apply
- **A.4 Rail-agnostic** (verify only the normalized shape), **A.7 Anti-replay** (rely on the
  on-chain proof-hash check; never settle a proof the chain already recorded),
  **A.3 PoV** (the chain decides finality), **A.1 Non-custodial**.

## Reuse first
- Signature/verify + hashing in `paylink-chain/internal/crypto`.
- The settlement tx type + proof-hash anti-replay in `paylink-chain/internal/chain/executor.go`.
- The lVM JSON-RPC client/methods in `paylink-chain/internal/rpc`.
- The Go/chi service conventions in [standard.md](../standard.md) (mirror work02's layout).
- **Imports `paylink-chain/internal/types` + `internal/crypto`** for byte-exact wire format.

## Acceptance criteria
- [ ] Valid proof → settlement tx broadcast → PayLink settles on-chain.
- [ ] Invalid signature/shape → rejected with error envelope; nothing broadcast.
- [ ] A proof already settled on-chain is not re-broadcast/double-settled (A.7).
- [ ] Tests ≥80%; lint/build clean; docker-compose entry healthy.
- [ ] Passes the Backend-service checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Adapter"/"Full stack": feed a signed proof, confirm
on-chain settlement via RPC; feed a tampered proof, confirm rejection.

## Notes / log
- Anti-replay is enforced on-chain — the service should defer to it, not duplicate the check
  as the source of truth.
