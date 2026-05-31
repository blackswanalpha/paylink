---
name: scaffold-adapter
description: Scaffold a new payment-rail adapter under adapters/ implementing the receive-callback to normalize-to-proof to sign to broadcast pipeline. Use when adding a payment rail (mpesa, card, bank, crypto). Emits the rail-agnostic proof shape and registers the adapter in the Payment Orchestrator config.
---

# Scaffold a LinkMint payment-rail adapter

Use this to create a new adapter under `adapters/<rail>/` that integrates an external payment
rail without ever leaking rail-specific detail past its boundary. **MPesa is the first rail
this phase** (work04); other rails are deferred (`workload/scope.md`) — scaffolding one is fine
but mark it deferred.

## The adapter pipeline (every adapter does exactly this)
```
receive rail callback  →  authenticate/verify  →  normalize to the proof shape
                       →  sign the proof        →  broadcast to the proof-validator
```

## What to generate
1. **Layout**
   ```
   adapters/<rail>/
     src/
       index.ts          # bootstrap + config (env only)
       receive.ts        # rail callback endpoint / listener + auth
       normalize.ts      # rail payload -> rail-agnostic proof
       sign.ts           # sign the proof (reuse chain crypto conventions)
       broadcast.ts      # POST proof to the proof-validator
     test/               # captured/representative callback fixtures -> end-to-end
     Dockerfile
     .env.example        # rail credentials documented, no real secrets
     README.md
   ```
2. **The proof shape** — `normalize.ts` must output EXACTLY:
   ```json
   { "pl_id": "...", "rail": "<rail>", "tx_id": "...", "amount": "...",
     "timestamp": "...", "sender": "...", "receiver": "...", "proof_signature": "..." }
   ```
   No rail-specific fields beyond this object.
3. **Registration** — add the adapter to the Payment Orchestrator config so it's discoverable.
4. **Wiring** — Dockerfile + docker-compose entry; secrets via env/KMS only.

## Invariants (workload/rules.md — critical for adapters)
- **A.1 Non-custodial** — funds move sender→receiver on the rail directly. **No LinkMint-owned
  wallet/account that holds or sweeps funds.** If the rail's design would require custody, stop
  and raise it.
- **A.4 Rail-agnostic** — nothing rail-specific crosses the `normalize.ts` boundary.
- **A.7 Anti-replay** — the on-chain proof-hash check is the source of truth; don't double-settle.
- Reuse signing/hashing conventions from `paylink-chain/internal/crypto`.

## Done
A captured callback flows end-to-end (replay → proof broadcast → PayLink settles on-chain via
RPC). Run `/security-review` (handles money + secrets). Close against the Adapter checklist in
`workload/definition-of-done.md`.
