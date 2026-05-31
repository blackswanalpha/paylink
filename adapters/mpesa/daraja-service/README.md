# mpesa-daraja rail service (work04)

The Node.js **Daraja rail SDK** for the MPesa adapter (ADR-007). It is the only component that
speaks Safaricom MPesa; it hands rail-neutral fields to the Go adapter core, which signs and
broadcasts the proof. Plain Node (ESM, Node ≥18) with **no runtime dependencies** — built-ins only
(`node:http`, `fetch`, `node:crypto`).

## Endpoints

| Method | Path | Caller | Auth | Purpose |
|--------|------|--------|------|---------|
| POST | `/stk` | the Go core | `X-Internal-Token` | OAuth + Lipa na M-Pesa STK push; returns `{checkout_request_id, …}` |
| POST | `/daraja/callback?t=<token>` | Safaricom | `?t=` token | parse the STK callback → forward rail-neutral fields to the core |
| GET | `/healthz`, `/readyz` | probes | — | liveness/readiness |

The STK push sets `BusinessShortCode`/`PartyB` to the **receiver's** shortcode (A.1 — funds settle
to the receiver, never a LinkMint account).

## Config

See `.env.example`. Daraja credentials (`DARAJA_CONSUMER_KEY/SECRET/PASSKEY`) are **secrets** —
env/KMS only. `DARAJA_STUB=true` runs a synthetic STK client (no live Safaricom calls) for
devnet/e2e. `DARAJA_SANDBOX=true` refuses a non-sandbox base URL.

## Run / test

```bash
node --test            # OAuth caching, STK body shaping, callback parse/forward, server routes
node src/index.js      # start (DARAJA_STUB=true to run without Daraja credentials)
```

Captured Daraja STK callback fixtures live in `test/fixtures/` (success + user-cancelled).
