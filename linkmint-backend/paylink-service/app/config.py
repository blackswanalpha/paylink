"""12-factor configuration — all values come from PAYLINK_* environment variables."""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import AliasChoices, Field, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="PAYLINK_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # HTTP
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8000
    log_level: str = "INFO"
    service_name: str = "paylink-service"

    # Persistence
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # lVM chain JSON-RPC
    chain_rpc_url: str = "http://localhost:8545/"
    chain_submit_enabled: bool = True
    signer_mode: Literal["service_key", "unsigned"] = "service_key"
    chain_signer_key: SecretStr | None = None

    # Domain defaults
    default_currency: str = "PLN"
    # Minor-unit amount above which the compliance/KYC gate (work12) applies on create.
    amount_kyc_threshold: int = 100_000_000

    # Compliance/KYC gate (work12). When enabled, a create whose amount exceeds
    # ``amount_kyc_threshold`` is synchronously evaluated by compliance-risk; a ``block`` decision
    # refuses creation with 402 KYC_REQUIRED before any row/chain tx (Flow E). The internal endpoint
    # is trusted-network; ``compliance_internal_token`` is the optional X-Internal-Token (ADR-009).
    # ``compliance_fail_open`` chooses behaviour when compliance-risk is unreachable: False
    # (default) fails closed (refuse above-threshold creation); True degrades open (allow + warn).
    compliance_check_enabled: bool = False
    compliance_service_url: str = "http://localhost:8093"
    compliance_internal_token: SecretStr | None = None
    compliance_timeout_seconds: float = 3.0
    compliance_fail_open: bool = False

    # In-app notification emit (FE work07). When enabled, PayLink created/verified/cancelled
    # transitions post a best-effort, address-scoped notification to notification-service's
    # trusted-network intake (``notify_internal_token`` is the optional X-Internal-Token, ADR-009).
    # Disabled by default so unit tests + minimal dev never make the outbound call.
    notify_enabled: bool = False
    notify_service_url: str = "http://localhost:8095"
    notify_internal_token: SecretStr | None = None
    notify_timeout_seconds: float = 3.0

    # Event publisher seam (work15). "log"/"noop" use the in-process LogPublisher/Noop; "kafka"
    # starts the outbox-drain relay that publishes paylink.paylink_events to the bus (ADR-011). The
    # durable outbox is always written in-transaction regardless of mode.
    event_publisher_mode: Literal["log", "noop", "kafka"] = "log"
    # Shared (unprefixed) bus brokers, matching eventbus-go / chain-event-mirror / docker-compose.
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))

    # Idempotency
    idempotency_ttl_seconds: int = 24 * 60 * 60

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
