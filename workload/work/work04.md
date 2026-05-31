# work04 — MPesa adapter (Daraja → normalized proof → sign → broadcast)

- **Status:** done · **Owner:** service-builder · **Stack:** Go/chi core + Node.js Daraja rail SDK (ADR-003 + **ADR-007**) · **Depends on:** 03 · **Flow:** [flow04](../flow/flow04.md)
- **Phase:** MVP (see [scope.md](../scope.md)) — **MPesa is the first and only rail this phase.**

## Goal
Build `adapters/mpesa` — integrate Safaricom Daraja so an MPesa payment for a PayLink is
received, normalized to the rail-agnostic proof, signed, and broadcast to the proof-validator.

## Why / context
The first real payment rail; proves the end-to-end non-custodial flow (`../../system.md`,
`../../deep-research-report.md` MPesa/Daraja integration). Sender pays the receiver directly
via MPesa; LinkMint only proves and settles.

## In scope
- Daraja integration: OAuth, STK push / C2B callback handling (sandbox first).
- Normalize the callback to `{pl_id, rail:"mpesa", tx_id, amount, timestamp, sender, receiver,
  proof_signature}` — **nothing MPesa-specific past this boundary.**
- Sign the proof and broadcast to the proof-validator (work03).
- Register the adapter in the Payment Orchestrator config.
- Tests with captured/representative callbacks; Dockerfile + docker-compose entry.

## Out of scope
- Card/bank/crypto adapters (deferred — see [scope.md](../scope.md)).
- Holding/forwarding funds (A.1 — funds move sender→receiver in MPesa directly).
- Proof verification/broadcast-to-chain logic (that's work03; the adapter calls it).

## Invariants that apply
- **A.1 Non-custodial** (no LinkMint-owned MPesa wallet sweeping funds — money is
  sender→receiver), **A.4 Rail-agnostic** (output only the proof shape).

## Reuse first
- Proof signing/hashing in `paylink-chain/internal/crypto`.
- The proof-validator API (work03) for broadcast.
- The adapter pipeline scaffolding via `/scaffold-adapter`.

## Acceptance criteria
- [x] Daraja sandbox callback received and authenticated (Node rail service; shared-token auth).
- [x] Mapped to the exact proof shape with `rail:"mpesa"`; no rail-specific leakage (golden + no-leak tests).
- [x] Proof signed and broadcast; the PayLink settles on-chain end-to-end (e2e test → VERIFIED + proof used).
- [x] Registered in the orchestrator config; tests pass; lint/build clean.
- [x] Passes the Adapter checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "Adapter" + "Full stack": replay a captured Daraja
callback → observe proof broadcast → confirm settled PayLink via RPC.

## Notes / log
- Secrets (Daraja consumer key/secret, passkey) via env/KMS only — never committed.
- **done (2026-05-29).** Hybrid build (**ADR-007**): a Go/chi **core** (`adapters/mpesa/`) owns
  normalize → sign (byte-exact via `paylink-chain/pkg/lvm`) → broadcast + `/v1/charges` +
  `/v1/callbacks/mpesa` + correlation (Redis) + idempotency; a Node.js **Daraja rail SDK**
  (`adapters/mpesa/daraja-service/`, built per the user's request) owns OAuth + STK push + raw
  callback parsing and forwards rail-neutral fields to the core. Signing stays in Go so the proof
  signature is byte-identical to what the validator trusts (devnet key `3f7a…a0f1` → pubkey
  `04e63cbe…` already in `PROOF_VALIDATOR_TRUSTED_PUBKEYS`). A.1: the STK push collects to the
  **receiver's** shortcode (`PartyB`); no LinkMint collection account exists. A.7: deterministic
  `Idempotency-Key` (`mpesa:<tx_id>`) + the validator/chain are the single dedupe authority.
  Orchestrator registration is **config-only** (`PAYMENT_ADAPTER_MPESA_URL`, logged at boot; the
  orchestrator does not call the adapter — rail stays opaque). Go core 75% cover, all unit + a
  server-level sign→verify end-to-end test pass; Node 13 `node:test` tests pass; chain + orchestrator
  still green. **Verified live** via `docker compose --profile e2e` (DARAJA_STUB=true, no Safaricom
  creds): charge → Daraja callback → Node → core → validator → PayLink **VERIFIED** on-chain + proof
  marked used. Deferred (follow-ups, not blocking): per-merchant Daraja credentials/shortcodes;
  Safaricom IP allowlist + separate callback/internal tokens; update the stale `scaffold-adapter`
  skill (still shows a TS layout) to Go/chi + Node-rail per ADR-003/ADR-007.
