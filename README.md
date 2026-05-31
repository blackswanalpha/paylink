# LinkMint

LinkMint implements the **PayLink protocol** — a non-custodial payment-coordination system that
links any payment rail (MPesa, card, bank, crypto) to on-chain logic. Senders pay directly via
their chosen rail; LinkMint only verifies proofs and finalizes settlement on the **lVM** chain.

- Architecture: [`system.md`](system.md) · Project guide: [`CLAUDE.md`](CLAUDE.md) · Work backlog: [`workload/backlog.md`](workload/backlog.md)

## Run the stack locally

The whole local stack runs from a single [`docker-compose.yml`](docker-compose.yml).

**Prerequisites:** Docker + Docker Compose v2 (`docker compose version`). To run a component's
tests outside containers you also need Go 1.25, Python 3.12, or Node 20 as appropriate.

### Default profile — app layer

```bash
docker compose up -d --build
```

Boots **Postgres + Redis + paylink-service + payment-orchestrator + api-gateway**. The gateway is
the front door on `http://localhost:8088`; `paylink-service` (`:8000`) and `payment-orchestrator`
(`:8080`) are also published. Without the chain, `payment-orchestrator`'s `/internal/readyz`
reports its chain dependency down by design (liveness stays up).

### e2e profile — full hybrid stack

```bash
docker compose --profile e2e up -d --build --wait
```

Additionally boots a single-validator **paylink-chain** devnet (`:8545`), **proof-validator**
(`:8081`), the **mpesa-adapter** Go core (`:8082`), and the **mpesa-daraja** Node rail
(`:8083`, `DARAJA_STUB=true`, so no Safaricom credentials are needed). `--wait` blocks until every
service's healthcheck passes.

| Service | URL | Profile |
|---|---|---|
| api-gateway (front door) | http://localhost:8088 | default |
| paylink-service | http://localhost:8000 | default |
| payment-orchestrator | http://localhost:8080 | default |
| paylink-chain (lVM JSON-RPC) | http://localhost:8545 | e2e |
| proof-validator | http://localhost:8081 | e2e |
| mpesa-adapter (core) | http://localhost:8082 | e2e |
| mpesa-daraja (rail) | http://localhost:8083 | e2e |

### The create → pay → settle flow

With the e2e stack up, the canonical scripted run is the end-to-end test:

```bash
make -C adapters/mpesa e2e        # go test -tags=e2e ./test/... -v
```

It exercises the full path (see [`adapters/mpesa/test/e2e_settlement_test.go`](adapters/mpesa/test/e2e_settlement_test.go)):

1. **Create** — mint a PayLink on-chain via the lVM RPC (`paylink_sendTransaction`); poll
   `paylink_getPayLink` until `CREATED`.
2. **Pay** — `POST /v1/charges` to the mpesa-adapter (`:8082`) triggers a (stubbed) STK push, then
   a Daraja success callback is posted to the rail: `POST :8083/daraja/callback?t=<token>`.
3. **Settle** — the adapter normalizes the callback to the rail-agnostic proof, signs it, and
   broadcasts via the proof-validator to the chain; poll `paylink_getPayLink` until `VERIFIED`
   (and `paylink_isProofUsed` is true).

### Tear down

```bash
docker compose --profile e2e down -v
```

`-v` drops the `pgdata` / `chaindata` volumes — required when changing the devnet genesis (stale
`chaindata` will mismatch a new genesis).

> **Local-only credentials.** `docker-compose.yml` contains inline devnet values — the dev JWT
> secret (`devsecret-change-me`), the partner API key, and deterministic devnet private keys (the
> chain validator key, the proof-validator signer, the adapter signer). These are **local-only
> fixtures** that make the single-validator e2e flow deterministic; they are not real credentials.
> Production uses RS256 + the identity service and KMS/Key-Vault-managed keys. Do not reuse them.

## CI

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) lints, builds, and tests every component on
each PR (Go services, Python service, Kong config, JS SDK, web app), with the full
`docker compose --profile e2e` settle smoke gated to `main` / an `e2e`-labelled PR. CI needs **no
secrets** — the rail runs with `DARAJA_STUB=true` and devnet keys are the local-only compose
fixtures above.

## Repository layout

See [`CLAUDE.md`](CLAUDE.md) for the full map. In short: `paylink-chain/` (lVM node, Go),
`linkmint-backend/` (services), `adapters/` (payment rails), `sdks/javascript/` (`@linkmint/sdk`),
`linkmint-frontend/` (web app), `workload/` (the work-item backlog driving development).
