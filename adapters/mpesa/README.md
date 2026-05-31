# mpesa adapter (work04)

The MPesa payment-rail adapter. It turns a completed Safaricom MPesa payment for a PayLink into a
signed, rail-agnostic proof and broadcasts it to the **proof-validator** (work03), which settles the
PayLink on-chain. Non-custodial: money moves payer → receiver on MPesa directly; LinkMint only
proves and settles.

```
payer pays via MPesa ─▶ Daraja STK callback
        │
        ▼
┌──────────────────────┐   rail-neutral fields   ┌───────────────────────┐  signed proof  ┌────────────────┐
│  daraja-service       │ ──────────────────────▶ │  mpesa-adapter (core) │ ─────────────▶ │ proof-validator │ ─▶ lVM settles
│  (Node.js rail SDK)   │   POST /v1/callbacks    │  (Go: normalize/sign/ │  POST /v1/proofs│   (work03)      │
│  OAuth · STK · parse  │                          │   broadcast)          │                └────────────────┘
└──────────────────────┘                          └───────────────────────┘
```

## Hybrid layout (ADR-007)

| Part | Lang | Owns |
|------|------|------|
| `./` (core) | Go/chi | normalize → **sign** (byte-exact via `paylink-chain/pkg/lvm`) → broadcast; `/v1/charges`, `/v1/callbacks/mpesa`; correlation (Redis); idempotency |
| `daraja-service/` | Node.js | the rail SDK: Daraja OAuth, STK push, raw STK-callback parsing; hands **rail-neutral** fields to the core |

The protocol-critical signing stays in Go so the proof signature is byte-identical to what the
validator trusts. Everything MPesa-specific (OAuth, STK wire shapes, callback JSON) lives in the
Node service and never crosses into the core beyond the neutral fields (invariant **A.4**).

## Flow

1. `POST /v1/charges` (core) `{pl_id, amount, payer_phone, receiver_shortcode?}` → core calls the
   rail service `POST /stk` → STK push to the **receiver's** shortcode → core stores the
   correlation `CheckoutRequestID → {pl_id, amount, receiver}` (Redis) → `202 {checkout_request_id}`.
2. Safaricom calls back the rail service `POST /daraja/callback?t=<token>` → it parses the MPesa
   envelope and forwards rail-neutral fields to the core `POST /v1/callbacks/mpesa` (internal token).
3. The core looks up the correlation, normalizes to the proof
   `{pl_id, rail:"mpesa", tx_id, amount, timestamp, sender, receiver, proof_signature}`, **signs** it,
   and broadcasts to the proof-validator. A re-delivered callback re-broadcasts with the same
   `Idempotency-Key` and the validator replays `already_settled` (anti-replay **A.7**).

## Invariants

- **A.1 Non-custodial** — the STK push collects to the *receiver's* shortcode (`PartyB`); there is no
  LinkMint-owned collection account anywhere in code or config. The adapter never sweeps/forwards
  funds — it only observes and proves.
- **A.4 Rail-agnostic** — only the 8-field proof crosses the core's boundary; MPesa shapes are
  confined to `daraja-service/`.
- **A.7 Anti-replay** — the proof-validator + the on-chain proof-hash check are the single dedupe
  authority; the adapter just uses a deterministic `Idempotency-Key` (`mpesa:<tx_id>`).

## Build / test / run

```bash
# Go core
go build ./... && go vet ./... && gofmt -l .
go test ./... -count=1 -cover         # unit + server end-to-end (faked Daraja + stub validator)
make cover                            # combined coverage
go run ./cmd/mpesa-adapter            # needs Redis + the rail service + the validator

# Node rail service
cd daraja-service && node --test      # OAuth cache, STK shaping, callback parse/forward, routes
node src/index.js                     # DARAJA_STUB=true for a no-credentials devnet run
```

End-to-end (full stack): `docker compose --profile e2e up -d --build`, then
`go test -tags=e2e ./test/... -v` (creates a PayLink, charges it, posts a stubbed Daraja callback,
asserts the PayLink reaches `VERIFIED` on-chain).

Config: `.env.example` (core) and `daraja-service/.env.example` (rail; **Daraja secrets live there**).
