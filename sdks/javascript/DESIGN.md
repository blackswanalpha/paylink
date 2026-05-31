# JS/TS SDK — design (work06)

The `@linkmint/sdk` is a thin, typed client over the LinkMint `/v1` API gateway. It is a **client
only**: no protocol logic, no funds, no rail knowledge. See [work06](../../workload/work/work06.md).

## Goals & constraints

- Strict TypeScript, **no `any`** ([standard.md](../../workload/standard.md) TS section).
- The SDK is the **contract** clients code against — fidelity to the wire shape beats ergonomics.
- **Invariant A.4 (rail-agnostic):** no rail-specific PayLink fields are exposed. The only rail
  reference is the opaque `PaymentRail` routing label used to initiate a payment.
- **Invariant A.1 (non-custodial):** the SDK moves no funds; `metadata`/`rules` are opaque and must
  not carry fund-moving data.
- Zero runtime dependencies; runs anywhere with a `fetch` (Node 18+, browsers, edge).

## Key decisions

1. **Mirror the wire shape exactly (snake_case).** `PayLink`, `Payment`, request bodies, and
   `next_cursor` use the server's field names verbatim (`pl_id`, `chain_tx_hash`, `paylink_id`, …).
   There is no camelCase mapping layer, so there is no mapping-bug surface; the types are sourced
   directly from `paylink-service/app/api/v1/schemas.py` and `payment-orchestrator`'s `payments.go`.
   SDK-native constructs (client options, error objects) use idiomatic camelCase.
2. **Single gateway `baseUrl`.** The client targets the Kong gateway (work05), which routes
   `/v1/paylinks*` → paylink-service and `/v1/payments*` → payment-orchestrator. Clients configure
   one URL.
3. **Auth is pass-through.** A discriminated `AuthConfig` sets `Authorization: Bearer <jwt>` or
   `X-API-Key: <key>`. The gateway verifies the credential and injects the caller's chain address
   (`X-Creator-Addr`) downstream — so the SDK deliberately never sends that header (ADR-006/008).
4. **Auto idempotency.** `payment-orchestrator` _requires_ `Idempotency-Key` on `POST /v1/payments`;
   paylink-service honors it when present. The SDK generates a UUID for every mutating call
   (overridable) so retries are safe everywhere.
5. **Typed errors from the envelope.** A single parser maps `{error:{code,message,details,trace_id}}`
   plus the response status and `X-Request-Id` into a `LinkMintApiError` hierarchy. Mapping is keyed
   on **HTTP status** (robust even for unknown codes) while `.code` stays a typed-but-open union.
   Non-envelope/JSON error bodies degrade gracefully into `details.body`.
6. **Injectable transport.** `fetch` and the timeout/abort plumbing are injectable, which makes the
   client testable against a mock `fetch` (a faithful `Response`-returning stub) with no live server.

## Layout

```
src/
  index.ts            public barrel
  client.ts           LinkMintClient + options + config resolution
  http.ts             transport: URL/query, auth, idempotency, timeout, error mapping
  errors.ts           LinkMintError hierarchy + envelope parsing + type guards
  idempotency.ts      UUID-v4 idempotency-key generator
  types.ts            wire DTOs (PayLink, Payment, enums, request/response shapes)
  resources/
    paylinks.ts       create / get / list / cancel
    payments.ts       initiate / get
test/                 vitest suites + a mock-fetch helper
```

## Out of scope (per the work item)

- Python/Go/Java/Flutter SDKs → work32 (Phase 3).
- Rail-specific helpers (the SDK stays rail-agnostic).
- A generated OpenAPI spec under `docs/api/` (the gateway's full `/v1` OpenAPI aggregation is a
  tracked follow-up of work05); these types are hand-mirrored from the services until then.
