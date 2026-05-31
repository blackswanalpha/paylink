# LinkMint Blockchain Feature Roadmap

Scope: the **lVM chain only** (`paylink-chain/`). For product-level features (escrow flows, rails, dashboards) see `prd.md`; for protocol design see `system.md`.

Each item below lists **What / Why / Where / Acceptance**. Tiers are ordered by deployment-blocking severity, not effort.

- **P0** — Consensus or data-integrity broken today. Devnet survives, anything beyond does not.
- **P1** — Production-hardening. Required before a public validator set.
- **P2** — Operational tooling. Required before external integrators can self-serve.
- **P3** — Core chain capabilities expected of a modern L1.
- **P4** — Advanced / differentiating. Cutting-edge or PayLink-specific.

Status legend: `missing` · `partial` (code exists but unwired) · `present` (working, may need hardening).

---

## P0 — Consensus & data integrity (must-fix before any multi-node deployment)

### P0.1 State persistence
- **What:** Persist `StateDB` (accounts, nonces, validators, PayLinks, total supply, burns) to BadgerDB so that restarts resume from the live state instead of genesis.
- **Why:** Verified live — sent a transfer, restarted the node on the same datadir, and the recipient balance reverted to 0, the admin balance reverted to the full initial supply, and the sender nonce reverted to 0. Block 1 is still in the block store, but `blockchain.Init` (`internal/chain/blockchain.go:31`) does not replay it, and `state.NewStateDB` (`internal/state/state.go`) initializes empty maps. Any restart causes immediate fork from peers and double-spend risk.
- **Where:** `internal/state/` (add KV-backed store), `internal/chain/blockchain.go:Init` (replay blocks or load a snapshot).
- **Acceptance:** A node restarted on a non-empty datadir reports the same `paylink_tokenStats`, `paylink_getAccount`, and `paylink_getPayLink` results as before shutdown. Integration test that sends a tx, restarts, and asserts state survives.

### P0.2 Transaction signature verification
- **What:** Reject any transaction whose `Signature` does not match `SignableBytes` under the public key that resolves to `tx.From`.
- **Why:** Verified live — submitted a `TxTransfer` with `"signature":""` and it landed in a block. `internal/chain/executor.go` has zero references to `Signature`; the helpers `crypto.Sign` / `Verify` / `VerifyWithAddress` (`internal/crypto/signing.go`) exist but are unused. Anyone can spend any account.
- **Where:** Add verification in `mempool.Add` (cheap reject) and re-check in `executor.ExecuteTx` before any state change.
- **Acceptance:** Unit test rejects forged signature; mempool drops it; executor returns receipt with `success: false, error: "invalid signature"`. RPC returns an error rather than a tx hash. Existing tests updated to sign their fixtures.

### P0.3 Block signature verification
- **What:** Verify `block.Commit.Signature` against `block.HeaderBytes()` using the proposer's registered key on every `AddBlock`.
- **Why:** `internal/chain/blockchain.go:58` (`AddBlock`) only checks height + prevHash continuity and recomputes the block hash. The P2P `OnBlock` callback in `cmd/paylinkd/main.go:171` passes peer blocks straight through. A malicious peer can inject blocks claiming any proposer.
- **Where:** `internal/chain/blockchain.go:AddBlock`. Resolve proposer pubkey from `validator_state.go`.
- **Acceptance:** Test inserts a block signed by the wrong key → rejected with `"invalid block signature"`. P2P fuzz test confirms forged blocks do not advance the tip.

### P0.4 VRF committee gating in the block producer
- **What:** `BlockProducer.produceBlock` must check VRF eligibility for the current height/round and skip the tick when not selected; the produced block must include the proposer's VRF proof.
- **Why:** `internal/consensus/committee.go` implements Algorand-style sortition fully, but `block_producer.go:88` proposes on every tick regardless. Any validator can produce blocks; "3-of-5 PoV" is unenforced.
- **Where:** `internal/consensus/block_producer.go` (gate on `CommitteeSelector.SelectCommittee`), `internal/types/block.go` (add `VRFProof` to header), `internal/chain/blockchain.go:AddBlock` (verify proof).
- **Acceptance:** Multi-validator integration test where only the VRF-selected proposer's blocks are accepted; others are rejected. Two-validator test confirms alternating proposers.

### P0.5 Quorum aggregation on blocks
- **What:** Block headers carry an aggregated 3-of-5 quorum signature from the elected committee; blocks without quorum are rejected.
- **Why:** `internal/consensus/quorum.go` collects votes but is never invoked by the producer. Single-proposer mode is currently the only path.
- **Where:** `internal/consensus/quorum.go` (drive collection), new vote-gossip topic in `internal/p2p/libp2p_host.go`, `block.Commit` upgraded to `[]Signature`.
- **Acceptance:** Block accepted only when ≥ `RequiredValidations` distinct committee members signed.

### P0.6 Fork choice & reorg
- **What:** Track candidate forks, accept the heaviest valid chain, and re-execute state on switch.
- **Why:** `blockchain.go:63` rejects any block not linking to `bc.tip` — linear append only. One invalid tip halts the chain; recovery requires manual datadir surgery.
- **Where:** New `internal/chain/forkchoice.go`; state needs deterministic re-execution from snapshots.
- **Acceptance:** Test where node receives competing valid chains and switches to the heavier; state on the abandoned branch is rolled back.

---

## P1 — Production hardening

### P1.1 Slashing wiring
- **What:** Detect double-sign and equivocation in the block-arrival path and emit `TxSubmitEvidence` automatically; apply stake reduction + jailing.
- **Why:** `internal/slashing/slashing.go` verifies evidence (line 116) and reduces stake, but nothing in the executor or producer ever *detects* misbehavior. Evidence only arrives via manual RPC.
- **Where:** Hook into `blockchain.AddBlock` (compare against alternative blocks at the same height); periodic liveness check in producer.
- **Acceptance:** Two validators sign two distinct blocks at the same height → automatic evidence tx → stake slashed within N blocks.

### P1.2 Mempool validation & ordering
- **What:** Mempool must reject txs that (a) fail signature, (b) have insufficient balance for fee + value, (c) have non-contiguous nonces beyond a configurable lookahead. `DrainForBlock` must return txs in a deterministic order (per-sender by nonce, across senders by fee/arrival).
- **Why:** `internal/txpool/mempool.go:32` only deduplicates by hash. `DrainForBlock` iterates an unordered map → non-deterministic block contents → state-root divergence across honest replays.
- **Where:** `internal/txpool/mempool.go`.
- **Acceptance:** Test asserts deterministic ordering across 1000 runs with the same input set. Fee/balance-insufficient tx is rejected at submission.

### P1.3 Genesis with multi-validator bootstrap
- **What:** Genesis schema supports an explicit `Validators[]` block with addresses, stakes, and VRF pubkeys.
- **Why:** `cmd/paylinkd/main.go:280` auto-generates a single-admin genesis tied to the proposer key. Spinning up a 5-validator testnet requires hand-editing JSON and hoping addresses match.
- **Where:** `internal/types/genesis.go`, `internal/chain/genesis.go`, new CLI subcommand `paylinkd genesis init --validators ...`.
- **Acceptance:** `paylinkd genesis init --validators val1:stake1,val2:stake2,...` produces a genesis file that boots a multi-validator devnet without manual editing.

### P1.4 Persistent state snapshots
- **What:** Periodic state snapshots written to BadgerDB; restart loads snapshot then replays only delta blocks.
- **Why:** Even with P0.1, full replay from genesis becomes prohibitive past 100k blocks.
- **Where:** `internal/state/snapshot.go` (new), `internal/chain/blockchain.go:Init`.
- **Acceptance:** Cold start time stays sub-second up to 1M blocks of history.

### P1.5 RPC unit tests
- **What:** Direct unit tests for every handler in `internal/rpc/handlers.go` against a mocked `Blockchain`/`StateDB`.
- **Why:** RPC is currently exercised only via `test/rpc_extended_test.go` integration. Refactors risk silent regressions.
- **Where:** `internal/rpc/handlers_test.go`.
- **Acceptance:** Each `paylink_*` method has at least happy-path + one error-path test.

### P1.6 Configurable chain parameters
- **What:** Move hard-coded constants (`maxTxsPerBlock = 500`, block interval defaults, mempool size, slashing percentages) into the genesis config and expose via `paylink_chainParams`.
- **Why:** Today these require recompilation to change.
- **Where:** `internal/types/genesis.go`, `internal/config/config.go`, threaded through producer/mempool/slashing.
- **Acceptance:** Two nodes with different binaries but the same genesis params agree on every block.

---

## P2 — Operational tooling

### P2.1 CLI client (`paylink`)
- **What:** Standalone CLI for key management, balance queries, tx construction/signing/broadcast, PayLink CRUD, stake/unstake, validator registration.
- **Why:** Today integrators must hand-craft JSON-RPC payloads.
- **Where:** New `cmd/paylink/main.go`.
- **Acceptance:** `paylink transfer --to 0x... --amount 1000` end-to-end without writing JSON.

### P2.2 CI/CD pipeline
- **What:** GitHub Actions running `go test ./... -race`, `golangci-lint`, `go vet`, plus binary releases on tag.
- **Where:** `.github/workflows/{test,lint,release}.yml`.
- **Acceptance:** PR cannot merge with failing tests; tagged releases produce signed binaries for linux/darwin × amd64/arm64.

### P2.3 Structured logging
- **What:** Replace `log.Printf` with `slog` (or zerolog) using structured fields; route through a configurable level and JSON formatter.
- **Why:** Current logs are unparseable by Loki/ELK.
- **Where:** Repo-wide.
- **Acceptance:** All logs parseable as JSON; PII-free; sampled at INFO by default.

### P2.4 Metrics expansion
- **What:** Add per-tx-type counters, mempool depth gauge, block-production latency histogram, peer-count gauge, slashing-events counter.
- **Where:** `internal/metrics/metrics.go`; emit from each touchpoint.
- **Acceptance:** Grafana dashboard renders all panels with non-zero data on a 3-node devnet under load.

### P2.5 Runbook & operator docs
- **What:** `docs/operations/` with: node-bootstrap, genesis-init, validator-onboarding, key-rotation, incident-response, slashing-recovery.
- **Where:** `paylink-chain/docs/`.

### P2.6 Chain explorer API
- **What:** Pagination, range queries, and address-indexed history on the RPC (`paylink_getTransactionsByAddress`, `paylink_getEventsByHeight`).
- **Why:** Current `getPayLinksByCreator`/`ByReceiver` return everything unpaginated.
- **Where:** `internal/rpc/handlers.go`, plus an indexer goroutine writing secondary indexes to BadgerDB.

---

## P3 — Core chain capabilities

### P3.1 On-chain governance
- **What:** New tx types: `TxProposeParamChange`, `TxVote`, `TxExecuteProposal`. Stake-weighted voting on chain parameters (fee bps, slashing %, block interval, treasury allocation).
- **Why:** Today every param change is a hard fork.
- **Where:** `internal/governance/` (new), `internal/types/transaction.go`.
- **Acceptance:** Validators can lower fee bps from 50 → 30 by majority stake vote without restarting nodes.

### P3.2 Coordinated upgrades / hard-fork scheduling
- **What:** Genesis & runtime carry an `Upgrades []{Name, ActivationHeight, ParamsDelta}` list; the executor applies rule changes at the activation height deterministically.
- **Where:** `internal/chain/executor.go` rule lookup keyed by height; `internal/rules/` versioned.
- **Acceptance:** Devnet upgrades fee schedule at a future height with no node restart.

### P3.3 Light client protocol
- **What:** Block headers + Merkle proofs for state queries; new RPC `paylink_getProof(address, key)`.
- **Why:** Mobile SDKs and rail adapters should not need a full archive node.
- **Where:** `internal/state/merkle.go` (extend), `internal/light/` (new server + client).
- **Acceptance:** SDK can verify a `getAccount` response without trusting the RPC endpoint.

### P3.4 Account abstraction (smart accounts)
- **What:** Allow `From` to be a script (multisig, threshold, time-lock, social recovery) rather than a single ECDSA address. New `TxRegisterAccount` defines the auth predicate.
- **Why:** Merchant treasuries and DAO-controlled receivers cannot be a single key.
- **Where:** `internal/accounts/` (new), executor signature check delegated to the account's predicate.
- **Acceptance:** A 2-of-3 multisig account can transfer funds when 2 signers cooperate; rejected with 1.

### P3.5 Native multisig & time-locks
- **What:** If P3.4 is too big, ship multisig and time-locked transfers as fixed account types first.
- **Where:** `internal/accounts/builtin.go`.
- **Acceptance:** PayLinks with time-locked receivers settle only after the lock expires.

### P3.6 Encrypted PayLink metadata
- **What:** PayLink `MetadataHash` resolves to encrypted-blob storage; only receiver + (optional) auditor key can decrypt.
- **Why:** Rail proofs leak counterparty data today.
- **Where:** New `internal/privacy/` plus an off-chain storage interface; chain stores only the hash + access-control list.
- **Acceptance:** Public RPC returns ciphertext; receiver SDK decrypts locally.

### P3.7 P2P transaction gossip + mempool sync
- **What:** Gossip-broadcast txs (not just blocks); new-peer mempool snapshot exchange.
- **Why:** `p2p` only broadcasts blocks today. Submitting a tx to one node may never reach the elected proposer.
- **Where:** `internal/p2p/libp2p_host.go` (new pubsub topic `txs`), `internal/txpool/mempool.go` (de-dup by hash on receive).

### P3.8 Validator key rotation
- **What:** `TxRotateValidatorKey` swaps a validator's signing key without losing stake or jailing.
- **Where:** `internal/types/transaction.go`, `internal/state/validator_state.go`.
- **Acceptance:** Validator rotates key at height N; blocks signed by the new key validate from N+1.

---

## P4 — Advanced & differentiating

### P4.1 BLS signature aggregation for committee votes
- **What:** Replace per-validator ECDSA signatures in block commits with a single BLS12-381 aggregate.
- **Why:** With a 100+ validator set, ECDSA commits balloon block headers and verification cost. BLS aggregation collapses N signatures into one ~96-byte signature with ~constant verification.
- **Where:** New `internal/crypto/bls.go`; vote/commit format upgrade gated by P3.2.
- **Acceptance:** Block header size grows < 100 bytes regardless of committee size.

### P4.2 Verifiable rail-proof oracle network
- **What:** First-class on-chain registry of rail adapters (`TxRegisterRail`), each with a signing key, supported proof format, and slashing bond. Validators verify rail proofs against the registry.
- **Why:** Today rail proofs are validated off-chain by a single Proof Validator service (per `system.md`). This is the chain's biggest centralization risk and the core invariant the PayLink protocol depends on.
- **Where:** `internal/rails/` (new); proof-validator service becomes a thin client.
- **Acceptance:** A misbehaving rail adapter (signing an unconfirmed MPesa tx) loses its bond automatically.

### P4.3 Encrypted mempool / threshold decryption
- **What:** Senders encrypt tx payloads to a threshold-decryption key held by the committee; payloads decrypt only after inclusion.
- **Why:** Eliminates front-running and selective censorship by proposers — material for a payments chain because settlement amounts are visible.
- **Where:** `internal/privacy/threshold/` (new); requires P4.1's BLS infra.
- **Acceptance:** A malicious proposer cannot inspect tx contents before committing to inclusion.

### P4.4 ZK proof verification primitives
- **What:** Precompiled verifiers for Groth16 / PLONK; new `TxSubmitZKProof` for off-chain compute attestations (private balance proofs, batched rail aggregations).
- **Where:** `internal/zk/` (new); integrate `gnark` library.
- **Acceptance:** A privacy-preserving "I-paid-X" proof verifies on-chain without revealing sender identity.

### P4.5 IBC-style cross-chain bridge
- **What:** Light-client verification of external chains (Ethereum, Stellar, Solana) for accepting cross-chain rail proofs and PLN bridging.
- **Where:** `internal/bridge/` (new).
- **Acceptance:** A USDC payment on Ethereum can settle a PayLink on lVM with on-chain proof verification.

### P4.6 Data-availability sampling
- **What:** Erasure-code block data; light clients sample chunks to detect withholding without downloading full blocks.
- **Where:** Block serialization upgrade; new sampling RPC. Material when block sizes grow past a few MB.

### P4.7 Recursive snapshot / state checkpoints with proofs
- **What:** Periodically Merkle-commit the full state and produce a succinct proof so that new nodes can join with a single proof verification rather than full replay.
- **Where:** `internal/state/`; integrates with P3.3 light clients.

### P4.8 MEV-aware ordering rules
- **What:** Define and enforce ordering policies (FIFO, batched-auction, or threshold-encrypted random) at the consensus layer rather than leaving it to proposer discretion.
- **Where:** `internal/consensus/ordering.go` (new).

### P4.9 HSM / hardware-wallet support for validator keys
- **What:** Validator signing via PKCS#11 / YubiHSM / Ledger; key never touches disk.
- **Where:** `internal/crypto/signer.go` interface with HSM-backed implementations.
- **Acceptance:** Validator runs end-to-end without ever exposing a raw private-key file.

### P4.10 Native subscriptions / streaming payments
- **What:** First-class recurring PayLinks settled per-block (Sablier/Superfluid-style streams) — directly maps to rent, payroll, SaaS.
- **Where:** New `TxCreateStream` + state extension.

### P4.11 Dispute & refund protocol
- **What:** On-chain dispute window post-settlement during which receivers can be challenged with counter-proofs (rail-reversal evidence); funds claw back if the challenge succeeds.
- **Where:** `internal/disputes/` (new); extend PayLink FSM with `DISPUTED` + `REFUNDED`.
- **Acceptance:** A chargeback on the underlying rail triggers automatic on-chain refund flow.

### P4.12 Insurance pool for failed-rail settlement
- **What:** Validators contribute to a pool that backstops PayLinks settled against rails that later reverse. Funded from a fee carve-out.
- **Where:** `internal/insurance/` (new); integrates with P4.11.

---

## Cross-cutting

### Test discipline
- Every P0/P1 item ships with a regression test that **would have caught the original gap** — e.g., the signature-verification work should include a test that submits an unsigned tx and asserts rejection.
- Add a `go test -race ./...` job and a long-running fuzz target for tx parsing and the executor.

### Determinism review
- Audit every executor branch for non-determinism (map iteration, time.Now, random IDs). The current `DrainForBlock` map iteration (P1.2) is one example; there are likely others.

### Threat model
- Publish `docs/security/threat-model.md` covering: signature forgery, replay across chains, long-range attack, censorship by majority, rail-adapter compromise, validator key extraction, governance capture.

---

## Recommended sequencing

1. **Stabilize devnet (1–2 weeks):** P0.1, P0.2, P0.3, P1.2, P1.3 — enough for a real multi-node testnet.
2. **Production-grade consensus (4–6 weeks):** P0.4, P0.5, P0.6, P1.1 — VRF + quorum + slashing actually wired.
3. **Operator readiness (2–3 weeks):** P2.1, P2.2, P2.3, P2.4, P2.5 — external integrators can run nodes.
4. **Differentiation (parallel, ongoing):** P3.1, P3.4, P4.2 in priority order — governance, smart accounts, and the rail-oracle network are the highest-leverage P3+ items for a payments chain.
