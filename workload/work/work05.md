# work05 — api-gateway (routing + auth)

> **Done** — Kong gateway shipped and verified; see the backlog changelog.

- **Status:** done · **Owner:** service-builder · **Stack:** Kong (DB-less declarative; **ADR-008** amends ADR-003) · **Depends on:** 09, 01 · **Flow:** [flow05](../flow/flow05.md)
- **Phase:** MVP (see [scope.md](../scope.md))

## Goal
Stand up `linkmint-backend/api-gateway` — a single ingress that routes `/v1/*` to the backend
services and enforces authentication (OAuth 2.0 / JWT for users, API keys for partners).

## Why / context
External clients (web app, SDK, partners) need one authenticated entry point rather than
talking to each service directly (`../../system.md` "API Gateway", `../../CLAUDE.md` Auth).
Each service kept an auth seam (work01) for the gateway to fill.

## In scope
- Route `/v1/paylinks*` → paylink-service, `/v1/payments*` → payment-orchestrator.
- Auth: validate JWTs (OAuth2) and API keys; reject with the standard error envelope.
- Rate limiting + request logging/correlation-id propagation.
- Kong config **or** a thin custom FastAPI gateway (decide in step 2; record as an ADR).
  Validates JWTs against identity-service (work09) and aggregates the `/v1` OpenAPI.
- Dockerfile + docker-compose entry; tests for routing + auth.

## Out of scope
- Implementing the identity provider / user store (use a documented OAuth2 issuer or a
  dev stub; full IdP is later).
- Business logic (lives in services).
- TLS termination / production ingress (local only this phase).

## Invariants that apply
- **A.1 Non-custodial** (gateway never touches funds), plus secrets handling from
  [rules.md](../rules.md) Part B (keys via env/KMS).

## Reuse first
- The `/v1` routing + error-envelope conventions in [standard.md](../standard.md).
- The auth seam left in work01/work02 services.

## Acceptance criteria
- [ ] `/v1/*` routes to the correct service; unknown routes → standard 404 envelope.
- [ ] Valid JWT / API key passes; invalid/missing → 401/403 with error envelope.
- [ ] Correlation IDs propagate downstream; basic rate limit enforced.
- [ ] Tests pass; lint/build clean; docker-compose entry healthy.
- [ ] Passes the relevant [definition-of-done.md](../definition-of-done.md) checklist(s).

## Verification
[verification.md](../verification.md) → "Full stack": call a routed endpoint with/without a
valid token; confirm routing + auth behavior and envelopes.

## Notes / log
- Record the Kong-vs-custom decision as an ADR in [decisions.md](../decisions.md).
