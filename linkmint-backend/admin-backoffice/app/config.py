"""12-factor configuration — all values come from ADMIN_* environment variables.

admin-backoffice is a read-only ops console. It is a JWT *consumer* (verifier-only) of
identity-service's RS256 tokens (``jwt_public_key_pem`` is identity's public key — there is no
signing key here) and it reads every entity through other services' HTTP APIs (no cross-schema DB
reads). It owns only a thin ``admin`` schema (staff scope grants + the Phase-2 flag/announcement
tables). Authorization is default-deny: a caller has only the scopes explicitly granted in
``admin.staff`` (or, for local dev, ``ADMIN_DEV_STAFF_GRANTS``).
"""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="ADMIN_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8092
    log_level: str = "INFO"
    service_name: str = "admin-backoffice"

    # ── Persistence (shared Postgres, own `admin` schema; Redis for readiness/Phase-2). ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM.
    # When unset, an ephemeral key is generated at startup so the verifier constructs — but NO real
    # token verifies (logged warning). JWKS-fetch is the documented follow-up (gateway precedent).
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── Upstream read APIs (the console aggregates these; internal network, no service-to-service
    # auth — the trusted-network precedent of payment-orchestrator → paylink-service). ──
    identity_service_url: str = "http://localhost:8090"
    merchant_service_url: str = "http://localhost:8091"
    paylink_service_url: str = "http://localhost:8000"
    payment_orchestrator_url: str = "http://localhost:8080"
    upstream_timeout_seconds: float = 5.0
    search_fanout_timeout_seconds: float = 4.0
    search_limit_default: int = 20

    # ── Audit sink (real audit-log-service is work13). log = one structured JSON line per
    # privileged access; noop = tests; http = POST each AuditRecord to audit-log-service
    # /v1/audit-log (the work13 drop-in). The http sink is best-effort + bounded-timeout: an
    # audit-log outage logs a warning, it never breaks an admin read. ──
    audit_sink_mode: Literal["log", "noop", "http"] = "log"
    audit_log_url: str = "http://localhost:8094"
    audit_internal_token: str | None = None  # X-Internal-Token for the work13 intake gate (ADR-009)
    audit_emit_timeout_seconds: float = 2.0

    # ── Authorization. Production grants live in ``admin.staff``; for local dev this CSV seeds
    # grants without touching the DB: "<sub>:scope,scope;<sub2>:scope". Default empty = locked. ──
    dev_staff_grants: str = ""

    def dev_staff_grant_map(self) -> dict[str, set[str]]:
        """Parse ADMIN_DEV_STAFF_GRANTS into ``{sub: {scope, ...}}`` (dev convenience only)."""
        grants: dict[str, set[str]] = {}
        for entry in self.dev_staff_grants.split(";"):
            entry = entry.strip()
            if not entry or ":" not in entry:
                continue
            sub, _, raw = entry.partition(":")
            scopes = {s.strip() for s in raw.split(",") if s.strip()}
            if sub.strip() and scopes:
                grants[sub.strip()] = scopes
        return grants


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
