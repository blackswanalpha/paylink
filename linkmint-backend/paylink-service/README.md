# paylink-service

The **first** LinkMint backend microservice and the **reference template** every other
Python/FastAPI service (work02/03/05/09–12/14) copies. It owns the off-chain `paylink` Postgres
schema, exposes versioned REST CRUD for PayLinks, and reflects on-chain settlement status by
reading the lVM JSON-RPC. Stack per **ADR-003** (Python/FastAPI).

## What it does

- `POST   /v1/paylinks`           — create a PayLink: persist the record **and** submit a signed
  `TxCreatePayLink` to the lVM, store `chain_tx_hash`. Requires `Idempotency-Key`.
- `GET    /v1/paylinks/{pl_id}`   — fetch one; status is reconciled from the chain (read-through).
- `GET    /v1/paylinks?creator=&receiver=&status=&limit=&cursor=` — list, cursor-paginated.
- `POST   /v1/paylinks/{pl_id}/cancel` — submit `TxCancelPayLink` (creator/owner only, `CREATED`
  only — mirrors the on-chain rules). Requires `Idempotency-Key`.
- `GET    /internal/healthz` · `/internal/readyz` · `/metrics`.

Every error uses the standard envelope: `{"error": {"code","message","details":{},"trace_id"}}`.

## Protocol invariants upheld

- **A.1 non-custodial** — stores metadata/state only; only a `MetadataHash` ever goes on-chain.
- **A.4 rail-agnostic** — no rail-specific fields in the model or API.
- **A.7 anti-replay** — settled status comes *only* from reconciling the chain, never invented.

## Run

```bash
pip install -e ".[dev]"
cp .env.example .env                       # edit as needed
alembic upgrade head                       # create the paylink schema/tables
uvicorn app.main:app --reload
```

Or the whole local stack from the repo root: `docker compose up -d` (add `--profile e2e` to also
boot a `paylink-chain` devnet for live read-through).

## Test / lint / type

```bash
ruff check . && black --check . && mypy .
pytest                                     # unit (no Docker) + integration (testcontainers); ≥80% coverage
```

Unit tests use mocks/fakeredis/httpx-mock and a FastAPI dependency-override harness (no Docker).
Integration tests (`-m integration`) spin real Postgres/Redis via testcontainers and are skipped
automatically when Docker is unavailable.

## Chain signing (seam)

The chain uses **NIST P-256** ECDSA; tx hash = `SHA256(SignableBytes)`; signature = raw `r||s`
(64 bytes, base64 on the wire); address = last 20 bytes of **legacy Keccak-256** of the
uncompressed pubkey. The chain does **not yet verify** tx signatures (ADR-005) — we sign correctly
for forward-compat, and `PAYLINK_SIGNER_MODE=unsigned` is a fallback. The signer is a swappable
seam (`app/chain/signer.py`) so a future client-signed flow (SDK/work05) can replace it.

## Deferred seams (not built here)

Real event transport → **work15**; background reconciliation worker / expiry sweeper; compliance
KYC gate → **work12**; auth/gateway → **work05**. All are marked `# SEAM (workNN)` in code.
