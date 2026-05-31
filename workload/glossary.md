# Glossary — LinkMint / PayLink / lVM terms

Short definitions for the domain language used across the codebase and these docs.
Sourced from `../system.md`, `../spec.md`, and `../CLAUDE.md`.

| Term | Meaning |
|------|---------|
| **PayLink** | An immutable, on-chain payment authorization that links a payment rail to on-chain settlement logic. The core protocol object. |
| **PayLink protocol** | The decentralized payment-coordination system LinkMint implements: create authorization → pay off-chain via a rail → prove → settle on-chain. |
| **lVM** | Link Virtual Machine — LinkMint's custom Go blockchain node (`paylink-chain/`). Native tx executor, no EVM, no smart contracts. |
| **paylinkd** | The lVM node binary / daemon. Run via `go run ./cmd/paylinkd` (see node flags in `../CLAUDE.md`). |
| **PoV (Proof-of-Validation)** | The consensus mechanism: a VRF-selected committee reaches quorum on discrete payment proofs, with immediate finality. |
| **ECVRF** | Elliptic-Curve Verifiable Random Function — used for verifiable, stake-weighted committee (sortition) selection. Implemented in `internal/crypto`. |
| **Committee / sortition** | The 3–5 validators VRF-selected per proof, Algorand-style, weighted by stake. |
| **Quorum (3-of-5)** | The threshold of committee validators that must agree for a proof to finalize. |
| **Validator** | A staked node that participates in committees, validates proofs, and earns minted PLN rewards. Has its own FSM (`internal/fsm`). |
| **Rail** | An external payment channel: `mpesa`, `card`, `bank`, or `crypto`. Adapters integrate rails. |
| **Adapter** | Code under `adapters/` that receives a rail callback, normalizes it to the proof format, signs it, and broadcasts it. |
| **Proof** | The rail-agnostic settlement evidence: `{pl_id, rail, tx_id, amount, timestamp, sender, receiver, proof_signature}`. |
| **Proof hash** | A hash of a proof stored on-chain to enforce anti-replay (one tx settles exactly one PayLink). |
| **Settlement** | Finalizing a PayLink on-chain once its proof reaches quorum. |
| **PLN** | The native utility token (staking, fees, governance). Managed in-state, **not** an ERC-20. |
| **Inflation fee model** | 0.5% settlement fee → 70% minted to validators / 20% treasury / 10% burned. |
| **Slashing** | Penalizing a validator for provable misbehavior; evidence detected/processed in `internal/slashing`. |
| **Double-entry ledger** | Append-only debit/credit recording of every monetary flow for audit and reconciliation. |
| **Anti-replay** | The guarantee that a given proof can settle only once, enforced by on-chain proof hashes. |
| **Datastream** | The WebSocket event stream (`internal/datastream`) emitting chain events to subscribers. |
| **Event bus** | In-process pub/sub (`internal/events`) that fans chain events out to consumers (RPC, datastream, metrics). |
| **GossipSub / DHT** | libp2p building blocks: GossipSub propagates blocks/txs; Kademlia DHT discovers peers. |
| **Non-custodial** | The invariant that LinkMint never holds user funds — money moves sender→receiver on the rail directly. |
| **State** | In-memory account/validator/PayLink state (`internal/state`) with a merkle root; persisted blocks live in BadgerDB (`internal/storage`). |
| **Work item / flow** | This process layer's unit of work: `work/workNN.md` (the contract) paired with `flow/flowNN.md` (the recipe). |
