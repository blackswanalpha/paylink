# proof-validator (work03)

The off-chain bridge between payment adapters and the lVM chain. It accepts a signed, rail-agnostic
payment proof over HTTP, verifies its shape and signature, and broadcasts a `TxSubmitValidation`
settlement transaction to the lVM JSON-RPC. In the MVP single-validator model it is the path from
"rail says paid" → "chain settles" (`system.md` §"Proof Validator").

Go/chi service; mirrors the work02 (`payment-orchestrator`) reference template. It reuses the lVM
wire format + crypto **byte-exact** via `paylink-chain/pkg/lvm` (imported through a `replace`
directive) — it never re-derives the transaction format.

## Protocol invariants

- **A.1 Non-custodial** — moves no funds; only verifies proofs and broadcasts a settlement tx.
- **A.3 PoV** — settlement finality is the chain's quorum decision; the service returns `202` and
  does not invent finality.
- **A.4 Rail-agnostic** — accepts only the normalized proof shape; `rail` is an opaque label.
- **A.7 Anti-replay** — defers to the on-chain proof-hash check (`paylink_isProofUsed`); a settled
  proof is never re-broadcast. A local `proof_hash` unique index is a complementary guard, not the
  source of truth.

## API

`POST /v1/proofs` (requires an `Idempotency-Key` header) — body is the proof shape:

```json
{
  "pl_id": "0x<64 hex>", "rail": "mpesa|card|bank|crypto", "tx_id": "<rail tx id>",
  "amount": 1500, "timestamp": 1730000000, "sender": "<rail id>", "receiver": "<rail id>",
  "proof_signature": "<base64 P-256 r||s>"
}
```

- `202 Accepted` `{proof_hash, tx_hash, status:"broadcast"}` — verified, settlement tx broadcast.
- `200 OK` `{proof_hash, status:"already_settled"}` — the chain already settled this proof (A.7).
- Errors use the standard envelope `{"error":{code,message,details,trace_id}}`:
  `INVALID_PROOF_SHAPE` (400), `INVALID_PROOF_SIGNATURE` (401), `PAYLINK_NOT_FOUND` (404),
  `PAYLINK_NOT_PAYABLE`/`PAYLINK_EXPIRED`/`PROOF_AMOUNT_MISMATCH` (409), `CHAIN_UNAVAILABLE` (502).

`GET /v1/proofs/{proof_hash}` — the stored record (status upgrades to `settled` once the chain
reports the proof used). `/internal/healthz`, `/internal/readyz`, `/metrics`.

## Configuration

Env only (12-factor) — see [`.env.example`](.env.example) for all `PROOF_VALIDATOR_*` keys.

## Build, test, run

```bash
make build         # go build ./...
make test          # unit tests (no Docker)
make cover         # unit + testcontainers integration; prints total coverage (DoD gate >=80%)
make lint          # go vet + gofmt check
make run           # go run ./cmd/proof-validator
```

`go mod tidy` and any build must run with `../../paylink-chain` present on disk (the `replace`
target). The Docker image builds from the **repo root** (it needs both module trees) — see the
Dockerfile header and `docker-compose.yml`.

## Local end-to-end (single-validator devnet)

```bash
docker compose --profile e2e up -d --build      # postgres + redis + paylink-chain + proof-validator
```

Devnet wiring (all values are well-known, non-secret — see `.env.example` and `genesis.devnet.json`):

- The chain runs with `--genesis genesis.devnet.json` (`requiredValidations: 1`) and `--privkey`
  set to the validator key `b71c…f291` (address `0xb186…51d2`), which the genesis pre-funds.
- The proof-validator signs settlement txs with the **same** key, so it is the single validator.
  With `PROOF_VALIDATOR_AUTO_STAKE=true` it stakes on startup and waits until active before serving;
  `/internal/readyz` reports `validator_active` so traffic only flows once it can settle.
- Trusted proof signer: the adapter key `3f7a…a0f1` (address `0x8aa5…34cb`); its public key is in
  `PROOF_VALIDATOR_TRUSTED_PUBKEYS`. The e2e signs proofs with this key.

> **Critical pairing:** the chain `--privkey` address, the genesis `adminAddress`/`initialBalances`,
> and `PROOF_VALIDATOR_CHAIN_SIGNER_KEY` must all be the same validator identity, and
> `PROOF_VALIDATOR_TRUSTED_PUBKEYS` must match the adapter key that signs the proofs. Use a fresh
> `chaindata` volume when changing the genesis (`docker compose --profile e2e down -v`).

See [`DESIGN.md`](DESIGN.md) for the proof-signature contract that adapters (work04) must follow.
