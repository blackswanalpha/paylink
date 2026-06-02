"""12-factor configuration — all values come from IDENTITY_* environment variables.

Secrets (JWT private key, MFA encryption key, OAuth client secrets) are typed ``SecretStr`` so they
never render in logs or reprs. Every key/secret defaults to *unset*; the security layer
auto-generates ephemeral dev material at startup so local dev is zero-config (mirrors the
paylink-service signer seam). Production injects the real material via env/KMS.
"""

from __future__ import annotations

from dataclasses import dataclass
from functools import lru_cache
from typing import Literal

from pydantic import AliasChoices, Field, SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict

OAuthProviderName = Literal["google", "apple", "github"]


@dataclass(frozen=True)
class OAuthProviderConfig:
    """Resolved per-provider OAuth credentials (None when unconfigured)."""

    provider: str
    client_id: str
    client_secret: str
    redirect_uri: str


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="IDENTITY_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8090
    log_level: str = "INFO"
    service_name: str = "identity-service"

    # ── Persistence (shared Postgres, own `identity` schema; Redis for idempotency) ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256). The signing key is NEVER in code — env/KMS only (rules.md B). When unset, an
    # ephemeral RSA-2048 keypair is generated at startup for local dev. The public key feeds the
    # JWKS endpoint + the gateway's additive RS256 consumer. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_private_key_pem: SecretStr | None = None
    jwt_public_key_pem: str | None = None
    access_token_ttl_seconds: int = 60 * 60  # 60-min access token (spec)
    refresh_token_ttl_seconds: int = 30 * 24 * 60 * 60  # 30-day refresh token

    # ── Password / API-key hashing (argon2id). Costs are tunable per environment. ──
    argon2_time_cost: int = 3
    argon2_memory_cost_kib: int = 64 * 1024
    argon2_parallelism: int = 4

    # ── MFA secret-at-rest encryption (KMS stand-in: AES-GCM with this env key). Ephemeral dev key
    # when unset. Rotating this re-encrypts mfa_factors (follow-up). ──
    mfa_encryption_key: SecretStr | None = None

    # ── Auth hardening ──
    auth_failed_threshold: int = 5  # emit identity.auth.failed after N consecutive failures

    # ── OAuth (stub + seam). oauth_fake=true selects the deterministic local fake provider
    # (verifiable without external creds — the work04 DARAJA_STUB analog). The real Google/Apple/
    # GitHub flows are config-driven via the per-provider creds below but aren't verified locally.
    oauth_fake: bool = False
    oauth_google_client_id: str = ""
    oauth_google_client_secret: SecretStr | None = None
    oauth_google_redirect_uri: str = ""
    oauth_apple_client_id: str = ""
    oauth_apple_client_secret: SecretStr | None = None
    oauth_apple_redirect_uri: str = ""
    oauth_github_client_id: str = ""
    oauth_github_client_secret: SecretStr | None = None
    oauth_github_redirect_uri: str = ""

    # ── Event publisher seam (work15). "log"/"noop" use the in-process LogPublisher/Noop; "kafka"
    # starts the outbox-drain relay that drains identity.identity_events to the bus (ADR-011). The
    # durable outbox is always written in-transaction regardless of mode.
    event_publisher_mode: Literal["log", "noop", "kafka"] = "log"
    # Shared (unprefixed) bus brokers, matching eventbus-go / chain-event-mirror / docker-compose.
    kafka_brokers: str = Field(default="", validation_alias=AliasChoices("KAFKA_BROKERS"))
    # work15 — bus consumer toggle (a lifespan task subscribes and calls the service's handle()).
    event_consumer_enabled: bool = False

    @property
    def kafka_broker_list(self) -> list[str]:
        return [b.strip() for b in self.kafka_brokers.split(",") if b.strip()]

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60

    def oauth_provider_config(self, provider: str) -> OAuthProviderConfig | None:
        """Resolve a provider's creds, or None when it isn't configured."""
        client_id = getattr(self, f"oauth_{provider}_client_id", "")
        secret = getattr(self, f"oauth_{provider}_client_secret", None)
        redirect = getattr(self, f"oauth_{provider}_redirect_uri", "")
        if not client_id:
            return None
        return OAuthProviderConfig(
            provider=provider,
            client_id=client_id,
            client_secret=secret.get_secret_value() if secret else "",
            redirect_uri=redirect,
        )


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
