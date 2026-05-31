# work06 — JS/TS SDK (typed `/v1` client)

> **Seeded** — expand with `/work 06` when picked up.

- **Status:** todo · **Owner:** service-builder · **Depends on:** 05 · **Flow:** [flow06](../flow/flow06.md)
- **Phase:** 2 / Beta (see [scope.md](../scope.md)) — **JS/TS SDK first; other languages are work32 (Phase 3).**

## Goal
Publish `sdks/javascript` — a typed client for the `/v1` API (PayLinks + payments) that the
web app and partners use instead of raw HTTP.

## Why / context
A clean SDK is the contract for clients and keeps API usage consistent (`../../CLAUDE.md`
SDKs, "Adding an API endpoint" workflow → "update SDK clients"). The web app (work07)
depends on it.

## In scope
- Typed methods for the `/v1/paylinks` and `/v1/payments` endpoints (create/get/list/cancel,
  initiate/status). Strict types, **no `any`**.
- Auth handling (bearer JWT / API key) passed through to the gateway.
- Maps the standard error envelope to typed errors.
- Unit tests against a mock server; covers success + error paths.

## Out of scope
- Python/Go/Java/Flutter SDKs (deferred).
- Bundling rail-specific helpers (rail-agnostic only).

## Invariants that apply
- **A.4 Rail-agnostic** (the SDK never exposes rail-specific PayLink fields),
  TS standards from [standard.md](../standard.md) (strict, no `any`).

## Reuse first
- The OpenAPI/endpoint definitions and error envelope from work01/work02/work05.
- Existing types mirrored from `paylink-chain/internal/types` where shared.

## Acceptance criteria
- [ ] Typed client covers all in-scope `/v1` endpoints; compiles in strict mode, no `any`.
- [ ] Passes auth through; surfaces the error envelope as typed errors.
- [ ] Tests cover success + error paths against a mock.
- [ ] Updated in lockstep with the endpoints it consumes.
- [ ] Passes the SDK checklist in [definition-of-done.md](../definition-of-done.md).

## Verification
[verification.md](../verification.md) → "SDK": run SDK tests; exercise create→read→settle
against the local stack.

## Notes / log
- Keep the SDK the single way clients call the API — discourage raw fetch in the web app.
