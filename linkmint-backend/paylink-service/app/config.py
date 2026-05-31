"""12-factor configuration — all values come from PAYLINK_* environment variables."""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import SecretStr
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
    # Minor-unit amount above which a future compliance/KYC gate (work12) would apply. Seam only.
    amount_kyc_threshold: int = 100_000_000

    # Event publisher seam (real Kafka/SQS transport deferred to work15). The durable outbox is
    # the paylink.paylink_events table, always written in-transaction by the service; this only
    # selects the live-notification seam.
    event_publisher_mode: Literal["log", "noop"] = "log"

    # Idempotency
    idempotency_ttl_seconds: int = 24 * 60 * 60


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
