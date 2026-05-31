# api-gateway (work05)

The single authenticated ingress for LinkMint. A **Kong** (DB-less, declarative) gateway that
routes `/v1/*` to the backend services and enforces authentication, rate limiting, correlation-id
propagation, and a trustworthy `X-Creator-Addr`.

Stack decision: **Kong**, recorded in [`workload/decisions.md`](../../workload/decisions.md) as
**ADR-008** (which amends ADR-003's Python/FastAPI classification for this one service).

## Routing

| Path | Upstream | Auth |
|------|----------|------|
| `/v1/paylinks*` | paylink-service (`:8000`) | required |
| `/v1/payments*` | payment-orchestrator (`:8080`) | required |
| anything else | — | `404` (LinkMint envelope) |

Paths are forwarded unchanged (`strip_path: false`). Unknown paths return the standard envelope
`{"error":{"code":"NOT_FOUND",...}}`.

## Auth

A request passes if it carries **either**:

- **JWT** — `Authorization: Bearer <token>` (OAuth2 users). Dev validates **HS256** tokens whose
  `iss` equals `GATEWAY_JWT_ISSUER`, signed with `GATEWAY_JWT_DEV_SECRET`. The caller's on-chain
  address is read from the `GATEWAY_JWT_CREATOR_ADDR_CLAIM` claim (default `creator_addr`).
- **API key** — `X-API-Key: <key>` (partners). The partner's on-chain address is its consumer
  `custom_id`.

Missing/invalid credentials → `401 UNAUTHORIZED` (envelope). All error responses (gateway- or
Kong-generated) are normalized to the LinkMint envelope; an upstream's own envelope is passed
through unchanged.

### identity-service (work09) seam
JWT validation is config-only. For production, register identity-service's **RS256** public key as
the `jwt_secret` (set `GATEWAY_JWT_ALGORITHM=RS256`) instead of the HS256 dev secret — no code
change. Dynamic JWKS fetch is a tracked follow-up.

## X-Creator-Addr (anti-spoofing, ADR-006)

The gateway is authoritative for caller identity. On every authenticated request it:
1. **strips** any client-supplied `X-Creator-Addr` / `X-Partner-Id`,
2. **injects** `X-Creator-Addr` = the verified JWT claim (or the API-key consumer's `custom_id`),
   lowercased,
3. **strips** the credentials (`Authorization`, `X-API-Key`) from the upstream hop.

paylink-service consumes this header via its existing `caller_address` seam.

## Health / metrics
- Liveness: `kong health` (container healthcheck).
- Metrics: Prometheus on the status listener (`:8100/metrics`).
- Admin API is bound to `127.0.0.1` and never exposed.

## Run

In the full local stack (from the repo root):
```bash
docker compose up -d            # boots postgres, redis, paylink-service, payment-orchestrator, api-gateway
# gateway is the front door on http://localhost:8088
```

Mint a dev token and call through the gateway:
```bash
TOKEN=$(python3 - <<'PY'
import jwt, time
print(jwt.encode({"iss":"linkmint-dev","sub":"u1","creator_addr":"0xabc",
                  "exp":int(time.time())+3600}, "devsecret-change-me", algorithm="HS256"))
PY
)
curl -s http://localhost:8088/v1/paylinks                       # 401 envelope
curl -s http://localhost:8088/v1/paylinks -H "Authorization: Bearer $TOKEN"   # routed
curl -s http://localhost:8088/v1/nope     -H "Authorization: Bearer $TOKEN"   # 404 envelope
```

## Test

Isolated acceptance matrix (gateway + Redis + echo upstreams; no real services needed):
```bash
make test        # boots test/docker-compose.test.yml, runs the pytest matrix, tears down
make validate    # render kong.yml + `kong config parse`
```

The suite (`test/test_gateway.py`) covers routing to each upstream, the 401/404/429 envelopes,
JWT + API-key auth, `X-Creator-Addr` injection/stripping, credential hygiene, correlation-id
propagation, and rate limiting. The upstream-down (`502`) check is opt-in (set
`GATEWAY_TEST_COMPOSE`).

## Files
- `kong/kong.yml.tmpl` — declarative config template (services, routes, consumers, plugins, and the
  serverless Lua for X-Creator-Addr injection + error-envelope normalization).
- `docker-entrypoint.sh` — renders the template from env (`envsubst`), then starts Kong.
- `Dockerfile` — `kong:3.7` + `gettext-base`.
- `test/` — isolated compose stack + echo upstream + pytest matrix.

## Deferred (tracked in the backlog)
Real partner-key store (rotation/revocation), dynamic JWKS against identity-service, full `/v1`
OpenAPI aggregation, payment-orchestrator binding to the injected `X-Creator-Addr`, the
`PAYLINK_REQUIRE_CREATOR_ADDR` enforcement flag in paylink-service, sliding-window rate limiting.
