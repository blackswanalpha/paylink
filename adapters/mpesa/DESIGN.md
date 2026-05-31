# mpesa adapter — design & the proof contract (work04)

## The proof contract (the adapter MUST reproduce it byte-for-byte)

The adapter produces the rail-agnostic proof and signs it. The `proof_signature` is an off-chain
trust contract between adapters (work04) and the proof-validator (work03): the chain never sees it,
only the derived on-chain `lvm.ProofHash`. To produce a valid proof, the **core**:

1. Builds the canonical bytes — `encoding/json.Marshal` (compact, HTML-escaped) of the proof
   **without** `proof_signature`, fields in this exact order:

   ```
   {"pl_id":"0x..","rail":"mpesa","tx_id":"..","amount":1500,"timestamp":1730000000,"sender":"..","receiver":".."}
   ```
   `amount` is a `uint64` (minor units); it equals the on-chain `PayLink.Amount` (the validator
   cross-checks). These bytes are byte-for-byte identical to the validator's `CanonicalBytes`,
   locked by a golden test (`internal/proof/proof_test.go`).

2. Signs `SHA256(canonical)` with its P-256 key → raw `r||s` (64 bytes) → `base64` → `proof_signature`
   (`internal/proof/sign.go`, reusing `paylink-chain/pkg/lvm` — same curve/encoding as the lVM tx
   signer). The signature verifies against the validator's `PROOF_VALIDATOR_TRUSTED_PUBKEYS`.

`tx_id` = the MPesa `MpesaReceiptNumber` (fallback `CheckoutRequestID`); `sender` = payer MSISDN;
`receiver` = the receiver shortcode. The validator does **not** cross-check sender/receiver (they are
rail identifiers, not on-chain addresses).

## Hybrid boundary (ADR-007): Go core + Node rail SDK

```
POST /v1/charges (core)
  └─ validate {pl_id, amount, payer_phone, receiver_shortcode?}; Idempotency-Key (Redis 24h)
  └─ rail.STKPush  ──HTTP /stk──▶  daraja-service (Node): OAuth + Lipa na M-Pesa STK push
  └─ correlation.Put(CheckoutRequestID → {pl_id, amount, receiver})            (Redis, TTL≈expiry)
  └─ 202 {checkout_request_id, status:"pending"}

Daraja ──POST /daraja/callback?t=token──▶ daraja-service (Node)
  └─ parse the STK callback envelope → rail-neutral fields
  └─ forward ──POST /v1/callbacks/mpesa (X-Internal-Token)──▶ core

POST /v1/callbacks/mpesa (core)
  └─ auth internal token
  └─ correlation.Get(CheckoutRequestID)            not found → ack (ignored_no_correlation)
  └─ ResultCode != 0                               → ack (ignored_failed_payment), no broadcast
  └─ paid amount != PayLink amount                 → ack (ignored_amount_mismatch), no broadcast (A.1)
  └─ normalize → proof.Validate → signer.Sign
  └─ broadcast ──POST /v1/proofs (Idempotency-Key: mpesa:<tx_id>)──▶ proof-validator
        202 broadcast / 200 already_settled → ack;  validator down → 5xx (Daraja redelivers)
```

Why split here: the proof signature must be byte-exact with what the validator trusts, which is
trivial in Go via `pkg/lvm`. So signing/broadcasting stays in Go; only the MPesa wire integration
moves to Node (the "rail SDK"). The A.4 boundary is the Node→core handoff of rail-neutral fields.

## Wire-format reuse (no re-derivation)

The core imports `paylink-chain/pkg/lvm` via a `replace` directive (`go.mod`) to sign byte-exact —
the same pattern as the proof-validator. Go's `internal/` rule forbids importing the chain's
`internal/*` cross-module, so the chain exposes the public `pkg/lvm` and the adapter depends on it.

## Anti-replay (A.7) & idempotency

The adapter does **not** keep its own dedupe ledger. It broadcasts with a deterministic
`Idempotency-Key` (`mpesa:<tx_id>`); the proof-validator's idempotency store + the on-chain
proof-hash check are the single authority. A re-delivered Daraja callback simply re-broadcasts and
gets `already_settled`.

## Non-custodial (A.1)

The STK push `BusinessShortCode`/`PartyB` is the **receiver's** shortcode (a per-charge parameter,
defaulting to the configured sandbox shortcode only when omitted). There is deliberately no
LinkMint-owned collection/pooling shortcode in code or config, and the adapter only performs STK
push + reads callbacks — never B2C/reversal/sweep. A payment whose amount differs from the PayLink
amount is **not** proved.

## Security notes (for `/security-review`)

- Daraja gives no callback signature; the callback URL is protected by a shared `?t=` token
  (constant-time compared), and the Node→core hop by `X-Internal-Token`. Production should add a
  Safaricom IP allowlist and rotate/separate the tokens.
- Daraja credentials (consumer key/secret, passkey) live only in `daraja-service` via env/KMS.
- Production must set real `MPESA_ADAPTER_SIGNER_KEY` / `MPESA_ADAPTER_INTERNAL_TOKEN` and
  `DARAJA_STUB=false`.
- Per-merchant shortcodes/passkeys are a real multi-merchant concern (sandbox uses a single pair);
  tracked as future work.
