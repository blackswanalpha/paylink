# admin-backoffice (work11)

Internal, **read-only** ops console for support / finance / compliance / engineering (Phase 1 of
`backendfeatures.md §2.18`). It concentrates entity inspection behind a single, fully-audited
surface. Mutating actions (suspend, force-refund, resolve, feature-flags, dual-approval) are
**Phase 2** and out of scope here.

## Endpoints (all require an admin JWT + MFA + the `support.read` scope; every access is audited)

| Method | Path | Purpose |
|---|---|---|
| GET | `/v1/admin/search?q=` | unified search across users, merchants, PayLinks, payments |
| GET | `/v1/admin/users/{id}` | user drill-down view |
| GET | `/v1/admin/merchants/{id}` | merchant drill-down view |
| GET | `/v1/admin/paylinks/{id}` | PayLink drill-down view |
| GET | `/v1/admin/payments/{id}` | payment drill-down view |
| GET | `/internal/healthz` · `/internal/readyz` · `/metrics` | liveness / readiness / Prometheus |

## How it works

- **Verifier-only JWT**: verifies identity-service's RS256 tokens (`ADMIN_JWT_PUBLIC_KEY_PEM`). It
  gates on the `admin` org/role, the `mfa` claim (identity emits it when login used a TOTP code),
  and a **default-deny** scope resolved from `admin.staff` (or `ADMIN_DEV_STAFF_GRANTS` for dev).
- **Read-through aggregation**: every entity is read over HTTP from its owning service's
  admin/internal endpoint (no cross-schema DB reads). One slow/down upstream degrades only its own
  search group — the response is still `200` with `degraded: [...]`.
- **Audit by construction**: the search/entity services are the only call sites and each emits one
  structured-JSON `audit` record. `audit-log-service` (work13) is a drop-in via `ADMIN_AUDIT_SINK_MODE`.
- **Owns only a thin `admin` schema**: `staff` (Phase-1 scope grants) plus the Phase-2
  `feature_flags`/`announcements` tables (spec fidelity; untouched in Phase 1).

## Develop

```bash
pip install -e ".[dev]"
ruff check . && black --check . && mypy .
pytest                       # unit + integration (testcontainers); 80% gate
uvicorn app.main:app --reload --port 8092
```

Grant yourself scopes for local dev without seeding the DB:
`ADMIN_DEV_STAFF_GRANTS=<your-jwt-sub>:superuser`. See the full end-to-end recipe (mint an MFA admin
token, hit each view) in the work11 plan / `workload/verification.md`.
