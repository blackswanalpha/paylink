"""12-factor configuration — all values come from MERCHANT_* environment variables.

Secrets (the bank-ref encryption key) are typed ``SecretStr`` so they never render in logs or
reprs. Every key/secret defaults to *unset*; the security layer auto-generates ephemeral dev
material at startup so local dev is zero-config (mirrors identity-service). Production injects the
real material via env/KMS.

merchant-onboarding is a JWT *consumer* (verifier-only): ``jwt_public_key_pem`` is
identity-service's RS256 public key, not a signing key. There is no private key here by design.
"""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="MERCHANT_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8091
    log_level: str = "INFO"
    service_name: str = "merchant-onboarding"

    # ── Persistence (shared Postgres, own `merchant` schema; Redis for idempotency) ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM
    # (served at identity's JWKS endpoint). When unset, an ephemeral RSA-2048 public key is
    # generated at startup so the verifier constructs — but NO real token verifies (logged warning).
    # The api-gateway precedent for the static-key/JWKS-fetch follow-up applies. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── Bank-ref-at-rest encryption (KMS stand-in: AES-GCM with this env key). Ephemeral dev key
    # when unset. Rotating this re-encrypts bank_accounts (follow-up). NEVER persist/log/return/emit
    # the plaintext account details. ──
    bank_encryption_key: SecretStr | None = None

    # ── Documents object store. local = filesystem (dev/test default) | s3 = AWS S3 (follow-up).
    # The DB stores only the s3_key; the bytes live in the object store. ──
    object_store_mode: Literal["local", "s3"] = "local"
    local_object_store_dir: str = "/tmp/merchant-documents"  # noqa: S108 - dev/test default only
    max_document_bytes: int = 10 * 1024 * 1024  # 10 MiB → 413 PAYLOAD_TOO_LARGE above this
    s3_bucket: str = ""
    s3_region: str = ""
    s3_endpoint_url: str = ""
    s3_prefix: str = "merchant-documents"

    # ── Onboarding policy (Phase 1 = single-country Kenya) ──
    allowed_countries: str = "KE"  # comma-separated ISO 3166-1 alpha-2 allowlist
    # Activation preconditions for decide(approve|reinstate) → ACTIVE (env-gated so unit tests can
    # exercise the bare state machine).
    require_verified_bank_for_active: bool = True
    require_contract_for_active: bool = True

    # ── Event publisher seam (real Kafka/SQS transport deferred to work15). The durable outbox is
    # the merchant.merchant_events table, written in-transaction; this only selects the live seam.
    event_publisher_mode: Literal["log", "noop"] = "log"

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60

    def allowed_country_set(self) -> set[str]:
        """Parse the country allowlist into an upper-cased set."""
        return {c.strip().upper() for c in self.allowed_countries.split(",") if c.strip()}


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
