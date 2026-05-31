# flow05 — api-gateway (execution recipe · seeded skeleton)

**Work item:** [work05](../work/work05.md) · **Goal recap:** authenticated single ingress
routing `/v1/*` to services.

## Pre-flight
- [x] Read [work05](../work/work05.md), [rules.md](../rules.md). work01 `done`; **work09 NOT done** → proceeded
  via a config-only JWT seam (dev HS256 / RS256-for-work09), which the work05 spec's "Out of scope" permits. Set `in-progress`.

## Steps (executed)

| # | Step | Agent / Skill | Outcome |
|---|------|---------------|---------|
| 1 | Inventory the `/v1` routes + auth seams of work01/work02 | **Explore** | `/v1/paylinks*` (work01, `X-Creator-Addr` seam in `deps.caller_address`), `/v1/payments*` (work02, no caller header) |
| 2 | Decide Kong vs custom gateway; design routing + auth | **Plan** → ADR | **Kong** (DB-less declarative) per owner → **ADR-008** (amends ADR-003) |
| 3 | Scaffold the gateway | Kong config | `linkmint-backend/api-gateway/` (`kong.yml.tmpl` + entrypoint + Dockerfile + Makefile) |
| 4 | Routing + JWT/API-key auth + `X-Creator-Addr` inject/strip + correlation-id + rate limit | service-builder | bundled plugins + one global serverless `post-function` |
| 5 | Tests for routing + auth (pass/fail/missing) | service-builder | isolated compose matrix (`test/`), **19/19** |
| 6 | Review vs invariants + `/security-review` (auth surface) | invariant-auditor + security review | invariant **PASS**; fixed 2 Medium credential-leak paths |
| 7 | Verify routing + auth on the stack | `/verify` | `kong config parse` + isolated matrix + live default-profile stack |

## Done
- [x] [work05](../work/work05.md) criteria met; DoD complete (Infra/CI + Universal); marked `done` in [backlog.md](../backlog.md).
