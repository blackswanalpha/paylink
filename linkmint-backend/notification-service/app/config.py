"""12-factor configuration — all values come from NOTIFY_* environment variables."""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import AliasChoices, Field, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="NOTIFY_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # HTTP
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8095
    log_level: str = "INFO"
    service_name: str = "notification-service"

    # Persistence
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    # Idempotency cache (Redis DB /0). Kept separate from the Celery broker (DB /1) so an eviction
    # policy / FLUSHDB on one never disturbs the other.
    redis_url: str = "redis://localhost:6379/0"

    # Celery / Redis broker (DB /1). ``celery_task_always_eager`` runs tasks inline (tests/dev).
    celery_broker_url: str = "redis://localhost:6379/1"
    celery_result_backend: str | None = None
    celery_task_always_eager: bool = False

    # Channels. ``console`` is the sandbox default (the verified Phase-1 path + the test double);
    # ``http`` swaps in the vendor drop-in (Africa's Talking / SendGrid). Secrets are SecretStr.
    sms_provider: Literal["console", "http"] = "console"
    email_provider: Literal["console", "http"] = "console"
    email_from: str = "no-reply@linkmint.local"
    # Comma-separated recipients the console provider force-fails,
    # so retry/backoff is exercised deterministically with no network.
    console_fail_recipients: str = ""

    # Africa's Talking (SMS) — config-gated http drop-in.
    africastalking_base_url: str = "https://api.africastalking.com/version1"
    africastalking_username: str = "sandbox"
    africastalking_api_key: SecretStr | None = None
    # SendGrid (email) — config-gated http drop-in.
    sendgrid_base_url: str = "https://api.sendgrid.com"
    sendgrid_api_key: SecretStr | None = None

    # Recipient resolution. ``inline`` (Phase-1 default) reads the contact off the trusted intake
    # call; ``identity`` (deferred seam) looks it up from identity-service so events carry no PII.
    recipient_resolver: Literal["inline", "identity"] = "inline"
    identity_service_url: str = "http://localhost:8090"
    identity_internal_token: SecretStr | None = None

    # Trusted-network gate (ADR-009). When set, POST /v1/notifications requires a constant-time
    # X-Internal-Token match; when unset, the deployment network is the only control.
    internal_shared_secret: SecretStr | None = None

    # In-app inbox (FE work07) auth seam: the gateway injects the authenticated caller as
    # X-Creator-Addr (mirrors paylink-service). When no gateway sits in front (local direct dev),
    # this optional fallback scopes the inbox; if neither is present, the read API returns 401.
    dev_creator_addr: str | None = None

    # Event publisher seam (forward-symmetry; emits no domain events in Phase 1).
    event_publisher_mode: Literal["log", "noop"] = "log"

    # work15 — bus consumer. When enabled, a lifespan task subscribes to the bus and feeds events to
    # NotificationEventConsumer.handle (the same chokepoint as the HTTP intake). Shared (unprefixed)
    # KAFKA_BROKERS matches eventbus-go / chain-event-mirror / docker-compose.
    event_consumer_enabled: bool = False
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))

    # Idempotency
    idempotency_ttl_seconds: int = 24 * 60 * 60

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]

    def console_fail_set(self) -> frozenset[str]:
        """Recipients the console provider force-fails (deterministic retry-test hook)."""
        return frozenset(r.strip() for r in self.console_fail_recipients.split(",") if r.strip())


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
