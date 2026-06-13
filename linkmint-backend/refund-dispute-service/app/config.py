"""12-factor configuration — all values come from REFUND_* environment variables.

Secrets (the internal token) are typed ``SecretStr`` so they never render in logs or reprs. Every
secret defaults to *unset*; the security layer auto-generates ephemeral dev material at startup so
local dev is zero-config (mirrors fee-pricing-service / compliance-risk). Production injects real
material via env/KMS.

refund-dispute-service is a JWT *consumer* (verifier-only): ``jwt_public_key_pem`` is identity's
RS256 public key, not a signing key. There is no private key here by design.
"""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import AliasChoices, Field, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="REFUND_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8100
    log_level: str = "INFO"
    service_name: str = "refund-dispute-service"

    # ── Persistence: shared Postgres (own `refund` schema); Redis (idempotency + consumer dedupe).
    # ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM.
    # When unset, an ephemeral RSA-2048 public key is generated at startup so the verifier
    # constructs — but NO real token verifies (logged warning). Mirrors fee-pricing-service. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── Platform-admin gate — comma-separated user_roles that may act across orgs. ──
    admin_user_roles: str = "admin"

    @property
    def admin_user_role_set(self) -> frozenset[str]:
        return frozenset(r.strip() for r in self.admin_user_roles.split(",") if r.strip())

    # ── Event publisher seam (work15). "log"/"noop" use the in-process LogPublisher/Noop; "kafka"
    # starts the outbox-drain relay that drains refund.refund_events to the bus (ADR-011). The
    # durable outbox is always written in-transaction regardless of mode. ──
    event_publisher_mode: Literal["log", "noop", "kafka"] = "log"
    # Shared (unprefixed) bus brokers, matching eventbus-go / chain-event-mirror / docker-compose.
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))
    # work15 — bus consumer toggle: a lifespan task subscribes to the `chain` topic and projects
    # chain.paylink.verified into verified_paylinks (the original-amount source of truth, A.3).
    event_consumer_enabled: bool = False

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]

    # ── Upstream services for refund eligibility + original-amount resolution. ──
    # payment-orchestrator: GET /v1/payments/{id} → {paylink_id, rail, status}. A refund is eligible
    # only when the payment exists and is SETTLED (the rail is captured for the reversal
    # instruction).
    payment_orchestrator_url: str = "http://localhost:8080"
    # paylink-service: GET /v1/paylinks/{id} → {amount, currency} — the original-amount fallback
    # when
    # the verified_paylinks projection has not yet seen the chain.paylink.verified event.
    paylink_service_url: str = "http://localhost:8000"
    upstream_timeout_seconds: float = 5.0

    # ── Amount validation. "strict": when neither the verified_paylinks projection nor
    # paylink-service
    # can resolve the original amount, reject with AMOUNT_SOURCE_UNAVAILABLE (502). "lenient":
    # accept
    # the caller-supplied amount (merchant approval is then the authorization gate). Production runs
    # strict (the chain mirror populates the projection); the dev compose runs lenient. ──
    amount_validation: Literal["strict", "lenient"] = "strict"

    # ── Rail reversal seam. No rail adapter supports reversal yet (mpesa = STK-push only;
    # card/crypto/
    # bank adapters are work28-30). So the reversal is instruction-only (A.1): the refund.reversal.
    # instructed event IS the instruction. When ``reversal_simulate`` is true the sweeper advances a
    # PROCESSING refund to COMPLETED after ``simulate_complete_after_seconds`` (dev/demo only). ──
    reversal_simulate: bool = False
    simulate_complete_after_seconds: int = 10

    # ── Dispute evidence window — the rail-imposed deadline to gather + submit evidence (work22). A
    # provider webhook may carry its own ``evidence_due_at``; this is the fallback. ──
    dispute_evidence_window_hours: int = 168

    # ── Provider webhook HMAC secrets, per provider: "<provider>:secret;<provider2>:secret". Parsed
    # by ``webhook_secrets_map``; the /v1/disputes/webhooks/{provider} route verifies X-Signature
    # over the raw body (constant-time HMAC-SHA256). ──
    webhook_secrets: str = "stub:devnet-dispute-secret"

    @property
    def webhook_secrets_map(self) -> dict[str, str]:
        """Parse REFUND_WEBHOOK_SECRETS into ``{provider: secret}``."""
        secrets: dict[str, str] = {}
        for entry in self.webhook_secrets.split(";"):
            entry = entry.strip()
            if not entry:
                continue
            provider, _, raw = entry.partition(":")
            provider = provider.strip()
            secret = raw.strip()
            if provider and secret:
                secrets[provider] = secret
        return secrets

    # ── Clawback coordination (work23 settlement). "event": write a refund.clawback.requested
    # outbox
    # row (the contract settlement will consume); "noop": skip (tests). The event IS the seam — no
    # synchronous coupling to a service that does not exist yet. ──
    clawback_mode: Literal["event", "noop"] = "event"

    # ── Sweeper (a lifespan task): expires OPEN disputes past their evidence window and (in dev)
    # simulates PROCESSING→COMPLETED. ──
    sweep_enabled: bool = True
    sweep_interval_seconds: int = 30

    # ── A.6 ledger-posting seam — OFF by default. Per-service ledger posting is deferred in work16
    # (the settlement hub, work23, is the intended writer); the seam honors A.6 intent without
    # coupling. ──
    ledger_posting_enabled: bool = False

    # ── Default currency when a refund request omits one. ──
    default_currency: str = "KES"

    # ── Trusted-network gate for any future /v1/internal/* surface (ADR-009). ──
    internal_shared_secret: SecretStr | None = None

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
