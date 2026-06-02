"""12-factor configuration — all values come from INVOICE_* environment variables.

Secrets (the paylink token) are typed ``SecretStr`` so they never render in logs or reprs. Every
secret defaults to *unset*; the security layer auto-generates ephemeral dev material at startup so
local dev is zero-config (mirrors compliance-risk). Production injects real material via env/KMS.

invoice-subscription is a JWT *consumer* (verifier-only): ``jwt_public_key_pem`` is identity's RS256
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
        env_prefix="INVOICE_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8096
    log_level: str = "INFO"
    service_name: str = "invoice-subscription"

    # ── Persistence (shared Postgres, own `invoice` schema; Redis for idempotency + dedupe) ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM.
    # When unset, an ephemeral RSA-2048 public key is generated at startup so the verifier
    # constructs — but NO real token verifies (logged warning). Mirrors compliance-risk. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── PayLink service (work01) — the aggregated PayLink is minted here on finalize. ──
    paylink_service_url: str = "http://localhost:8000"
    paylink_internal_token: SecretStr | None = None
    paylink_timeout_seconds: float = 5.0

    # ── Default settlement currency when the request omits one. ──
    default_currency: str = "PLN"

    # ── Event publisher seam (work15). "log"/"noop" use the in-process LogPublisher/Noop; "kafka"
    # starts the outbox-drain relay that drains invoice.invoice_events to the bus (ADR-011). The
    # durable outbox is always written in-transaction regardless of mode. ──
    event_publisher_mode: Literal["log", "noop", "kafka"] = "log"
    # Shared (unprefixed) bus brokers, matching eventbus-go / chain-event-mirror / docker-compose.
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))
    # work15 — bus consumer toggle: a lifespan task subscribes to chain.* and marks invoices PAID.
    event_consumer_enabled: bool = False

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]

    # ── Overdue sweeper (a lifespan task flips OPEN→OVERDUE past due_at; emits invoice.overdue). ──
    overdue_sweep_enabled: bool = True
    overdue_sweep_interval_seconds: int = 60

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
