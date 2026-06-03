"""12-factor configuration — all values come from PRICING_* environment variables.

Secrets (the internal token) are typed ``SecretStr`` so they never render in logs or reprs. Every
secret defaults to *unset*; the security layer auto-generates ephemeral dev material at startup so
local dev is zero-config (mirrors invoice-subscription / compliance-risk). Production injects real
material via env/KMS.

fee-pricing-service is a JWT *consumer* (verifier-only): ``jwt_public_key_pem`` is identity's RS256
public key, not a signing key. There is no private key here by design.
"""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import AliasChoices, Field, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="PRICING_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8097
    log_level: str = "INFO"
    service_name: str = "fee-pricing-service"

    # ── Persistence: shared Postgres (own `pricing` schema); Redis (idempotency + FX hot cache). ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM.
    # When unset, an ephemeral RSA-2048 public key is generated at startup so the verifier
    # constructs — but NO real token verifies (logged warning). Mirrors invoice-subscription. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── Platform-admin gate for /v1/pricing/tiers — comma-separated user_roles allowed. ──
    admin_user_roles: str = "admin"

    @property
    def admin_user_role_set(self) -> frozenset[str]:
        return frozenset(r.strip() for r in self.admin_user_roles.split(",") if r.strip())

    # ── Event publisher seam (work15). "log"/"noop" use the in-process LogPublisher/Noop; "kafka"
    # starts the outbox-drain relay that drains pricing.pricing_events to the bus (ADR-011). The
    # durable outbox is always written in-transaction regardless of mode. ──
    event_publisher_mode: Literal["log", "noop", "kafka"] = "log"
    # Shared (unprefixed) bus brokers, matching eventbus-go / chain-event-mirror / docker-compose.
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))
    # work15 — bus consumer toggle: a lifespan task subscribes to the `merchant` topic and keeps the
    # local merchant_pricing tier cache in sync (merchant.onboarded / merchant.fee_tier.changed).
    event_consumer_enabled: bool = False

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]

    # ── FX provider seam. "static" reads deterministic rates from env; "http" uses the live seam.
    # Rates are cached in Redis for `fx_cache_ttl_seconds` and LOCKED into each quote for audit. ──
    fx_provider: Literal["static", "http"] = "static"
    fx_cache_ttl_seconds: int = 60
    # "USD:KES=129.50,EUR:KES=140.00,KES:KES=1" — base:quote=rate, comma-separated.
    fx_static_rates: str = "USD:KES=129.50,EUR:KES=140.00,KES:KES=1"
    # Optional fallback rates used when the provider returns nothing (same format; source=fallback).
    fx_fallback_rates: str = ""
    # Live FX provider base URL (only used when fx_provider="http").
    fx_http_url: str = ""
    fx_http_timeout_seconds: float = 5.0

    # ── Monthly platform-fee invoice sweeper (a lifespan task; generation is idempotent per
    # merchant+period). The internal run endpoint triggers the same path on demand. ──
    invoice_sweep_enabled: bool = True
    invoice_sweep_interval_seconds: int = 3600

    # ── Trusted-network gate for /v1/internal/* (ADR-009). When unset, the trusted network is the
    # only control; when set, a constant-time X-Internal-Token match is also required. ──
    internal_shared_secret: SecretStr | None = None

    # ── Optional accrual-from-events seam: when true the bus consumer also subscribes to `chain`
    # and records a platform-fee accrual on chain.paylink.verified. Default OFF (the load-bearing
    # accrual path is the internal HTTP endpoint; the fee-bearing producing event isn't fixed). ──
    accrual_from_events: bool = False

    # ── A.6 ledger-posting seam — OFF by default. Per-service ledger posting is deferred in work16
    # (the settlement hub is the intended writer); the seam honors A.6 intent without coupling. ──
    ledger_posting_enabled: bool = False

    # ── Default settlement currency when a quote request omits one. ──
    default_currency: str = "KES"

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
