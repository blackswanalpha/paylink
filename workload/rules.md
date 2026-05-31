# Rules — protocol invariants & how Claude operates here

Two kinds of rules live here. **Part A** are protocol invariants: violating one is a
defect even if the code compiles and tests pass. **Part B** is how Claude should work
in this repository. When a request conflicts with Part A, stop and surface the conflict
rather than implementing it.

---

## Part A — Protocol invariants (never violate)

These are the 8 "Key Architecture Decisions" from [`../CLAUDE.md`](../CLAUDE.md),
restated as hard rules. They are the contract the whole system rests on.

1. **Non-custodial.** The protocol NEVER holds, escrows in its own name, sweeps, or
   takes title to user funds. Money flows sender → receiver over the external rail.
   LinkMint only verifies proofs and finalizes settlement on-chain. *This is a legal
   requirement, not a preference.* Any design that routes funds through a LinkMint-owned
   account, wallet, or contract is rejected.

2. **Custom chain, no EVM.** All protocol logic is native Go in `paylink-chain/`. There
   are **no smart contracts, no Solidity, no EVM bytecode**. "Deploy a contract" is never
   the answer — add logic to the executor instead (see the tx-type recipe in
   [`standard.md`](standard.md)).

3. **Proof-of-Validation consensus.** VRF-based, stake-weighted committee selection
   (Algorand-style sortition); quorum (3-of-5) on discrete payment proofs; immediate
   finality. Don't replace with PoW/PoS or weaken the quorum.

4. **Rail-agnostic proof format.** Every adapter normalizes to exactly:
   `{pl_id, rail, tx_id, amount, timestamp, sender, receiver, proof_signature}`.
   Core services and the chain are rail-unaware. Don't leak rail-specific fields past
   the adapter boundary.

5. **PLN inflation fee model.** 0.5% fee on settlement, split **70% minted to validators
   / 20% treasury / 10% burned**. No upfront deposits. Don't change the split or
   introduce pre-funding without an ADR.

6. **Double-entry ledger.** Every monetary flow is recorded with matching debit/credit
   entries. Ledger tables are append-only — corrections are new entries, never edits.

7. **Anti-replay.** Proof hashes are stored on-chain in state. **One transaction settles
   exactly one PayLink.** Never allow a proof hash to settle twice.

8. **P2P mesh (lVM network).** libp2p with GossipSub (block/tx propagation), Kademlia DHT
   (peer discovery), and a block-sync protocol for new nodes. Don't introduce a central
   relay that bypasses the mesh.

> If a task seems to require breaking one of these, it's either out of scope
> ([`scope.md`](scope.md)) or it needs an ADR in [`decisions.md`](decisions.md) first.

---

## Part B — How Claude operates in this repo

**Plan before building.** For anything beyond a trivial edit, understand the existing
code first (the chain is large and already implements most primitives), then act.

**Reuse first.** `paylink-chain/internal/` already has crypto, state, fee, rules, fsm,
events, rpc, p2p. Search for an existing function/type before writing a new one. New code
should read like the code around it (naming, comments, idiom).

**Stay in scope.** Work the active `workNN.md` item only. Discovered side-work goes to
[`backlog.md`](backlog.md) as a new item — it does not expand the current one.

**Verify your work.**
- After any chain change: `cd paylink-chain && go build ./... && go test ./... -count=1`.
- After a TS service change: build + unit tests (Jest/Vitest), targeting 80% coverage.
- Match the change type's checklist in [`definition-of-done.md`](definition-of-done.md).

**Commits & branches.** Conventional Commits with scope (`feat(paylink-chain): …`,
`fix(mpesa): …`). Branch model: `main` / `develop` / `feature/*` / `fix/*` / `release/*`.
This repo is not yet a git repo — `git init` before the first commit (ask first).

**Secrets.** Never commit secrets. Use env vars or vault refs (AWS KMS / Azure Key Vault).
`.env` stays in `.gitignore`.

**Outward-facing / destructive actions** (publishing, deploying, deleting, network calls
to third parties like Daraja) require explicit confirmation unless already authorized.

**Faithful reporting.** If tests fail, say so with output. If a step was skipped, say so.
Don't claim "done" without the verification to back it.
