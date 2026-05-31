# proof-validator — design & the proof contract (work03)

## Flow

```
POST /v1/proofs
  └─ Idempotency-Key (Redis 24h)            replay → cached; key+different body → 409
  └─ proof.Parse                            strict JSON (amount accepts number or "number")
  └─ proof.ValidateShape                    A.4 gate → INVALID_PROOF_SHAPE (nothing broadcast)
  └─ verifier.Verify(proof_signature)       trusted P-256 keys → INVALID_PROOF_SIGNATURE
  └─ proofHash = lvm.ProofHash(plID,txId,amount)
  └─ chain.IsProofUsed(proofHash)           A.7 → if used: status=already_settled (no broadcast)
  └─ cross-check chain.GetPayLink           status=CREATED, amount==pl.Amount, not expired
  └─ store.InsertProof(status=received)     unique proof_hash = local double-broadcast guard
  └─ nonce.Reserve → lvm.BuildSubmitValidationTx → signer.SignTx → chain.SendTransaction
  └─ store.MarkBroadcast(tx_hash) → 202 {proof_hash, tx_hash, status:broadcast}
```

`TxSubmitValidation` is a *vote*; the chain settles the PayLink (→ `VERIFIED`, marks the proof
used) when `voteCount >= RequiredValidations` (1 in the single-validator devnet). The validator
does not wait for finality — the orchestrator (work02) observes the settlement event.

The chain settles on status/expiry/proof-usage but does **not** check the proof's amount, so the
cross-check (`PROOF_VALIDATOR_PAYLINK_CROSSCHECK=true`) is where the off-chain bridge enforces
`amount == PayLink.amount`. The proof's `sender`/`receiver` are rail identifiers (e.g. phone
numbers), not on-chain addresses, so they are **not** cross-checked against the PayLink receiver.

## The proof contract (adapters / work04 MUST reproduce byte-for-byte)

The `proof_signature` is an off-chain trust contract between adapters and this validator. The chain
never sees it — only the derived `proofHash` goes on-chain. To produce a valid proof, an adapter:

1. Builds the canonical bytes — `encoding/json.Marshal` (compact, HTML-escaped) of the proof
   **without** `proof_signature`, fields in this exact order:

   ```
   {"pl_id":"0x..","rail":"mpesa","tx_id":"..","amount":1500,"timestamp":1730000000,"sender":"..","receiver":".."}
   ```
   `amount` is a `uint64` (minor units). It matches the on-chain `PayLink.Amount`.

2. Signs `SHA256(canonical)` with its P-256 key → raw `r||s` (64 bytes) → `base64` standard
   encoding → `proof_signature`. (Same curve/encoding as the lVM tx signer.)

3. The validator verifies that signature against `PROOF_VALIDATOR_TRUSTED_PUBKEYS` (uncompressed
   P-256 hex). Empty list ⇒ fail-closed (every proof rejected).

The on-chain identity is `lvm.ProofHash(plID, txId, amount) = SHA256(go_json({paylinkId,txId,amount}))`
(see `paylink-chain/pkg/lvm`). It binds the PayLink, the rail tx id, and the amount; it is the A.7
dedupe key and is identical across every component (validator + adapters) because they all import
`lvm.ProofHash`.

## Wire format reuse (no re-derivation)

`paylink-chain/pkg/lvm` re-exports the chain's `internal/{types,crypto}` (type aliases + thin
wrappers) so this service constructs/signs `TxSubmitValidation` byte-exact. Go's `internal/` rule
forbids importing those packages cross-module, so the chain exposes the public `pkg/lvm` and the
service depends on it via a `replace` directive — one source of truth, not a second implementation.

The chain does not yet verify tx signatures (ADR-005); the service signs anyway (forward-compat)
and the From-derived hash matches what `paylink_sendTransaction` recomputes.

## Single-validator settlement (devnet)

`TxSubmitValidation` is only accepted from an **active validator**. The devnet genesis seeds none
and defaults to `RequiredValidations: 3`, so work03 ships `genesis.devnet.json`
(`requiredValidations: 1`, validator pre-funded) and a devnet-flagged auto-stake: on startup the
service stakes `minimumStake` and waits until active before serving. Production keeps auto-stake off.
