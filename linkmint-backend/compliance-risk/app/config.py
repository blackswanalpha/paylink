"""12-factor configuration — all values come from COMPLIANCE_* environment variables.

Secrets (callback HMAC secrets, the internal shared secret, the provider-ref encryption key) are
typed ``SecretStr`` so they never render in logs or reprs. Every secret defaults to *unset*; the
security layer auto-generates ephemeral dev material at startup so local dev is zero-config (mirrors
identity-service). Production injects the real material via env/KMS.

compliance-risk is a JWT *consumer* (verifier-only): ``jwt_public_key_pem`` is identity-service's
RS256 public key, not a signing key. There is no private key here by design.
"""

from __future__ import annotations

from decimal import Decimal
from functools import lru_cache
from typing import TYPE_CHECKING, Literal

from pydantic import SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict

if TYPE_CHECKING:
    from app.domain.risk_engine import RiskConfig


class Settings(BaseSettings):
    """Service configuration. No secrets in code — everything is env-sourced."""

    model_config = SettingsConfigDict(
        env_prefix="COMPLIANCE_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # ── HTTP ──
    http_host: str = "0.0.0.0"  # noqa: S104 - containerized service binds all interfaces
    http_port: int = 8093
    log_level: str = "INFO"
    service_name: str = "compliance-risk"

    # ── Persistence (shared Postgres, own `compliance` schema; Redis for idempotency) ──
    database_url: str = "postgresql+psycopg://paylink:paylink@localhost:5432/paylink"
    redis_url: str = "redis://localhost:6379/0"

    # ── JWT (RS256, verify-only). The public key is identity-service's SubjectPublicKeyInfo PEM
    # (served at identity's JWKS endpoint). When unset, an ephemeral RSA-2048 public key is
    # generated at startup so the verifier constructs — but NO real token verifies (logged warning).
    # The api-gateway precedent for the static-key/JWKS-fetch follow-up applies. ──
    jwt_issuer: str = "linkmint-identity"
    jwt_audience: str = "linkmint"
    jwt_public_key_pem: str | None = None

    # ── KYC provider. stub = in-process fake (MVP default + tests) | http = real vendor drop-in. ──
    kyc_provider: Literal["stub", "http"] = "stub"
    kyc_provider_url: str = ""
    kyc_validity_days: int = 365

    # ── KYC callback HMAC secrets, per provider: "<provider>:secret;<provider2>:secret". Parsed by
    # ``callback_secrets_map()``; the callback route verifies X-Signature over the raw body. ──
    callback_secrets: str = "stub:devnet-callback-secret"

    # ── Internal endpoint defense-in-depth (ADR-009). When set, /v1/risk/evaluate additionally
    # requires a constant-time X-Internal-Token match; when unset, the trusted network is the only
    # control (mpesa precedent). ──
    internal_shared_secret: SecretStr | None = None

    # ── Provider-ref-at-rest encryption (KMS stand-in: AES-GCM with this env key). Ephemeral dev
    # key when unset. Envelopes ``kyc_records.provider_ref``. NEVER persist/log/return/emit PII. ──
    provider_encryption_key: SecretStr | None = None

    # ── Risk knobs (env-tunable; consumed by the pure risk engine). ──
    tier1_ceiling: Decimal = Decimal("50000")
    # Empty string → +Infinity (tier 2 has no per-tx ceiling). Parsed in ``risk_config()``.
    tier2_ceiling: str = ""
    aml_cumulative_threshold: Decimal = Decimal("150000")
    aml_window_hours: int = 720  # 30 days
    velocity_block_24h: int = 50
    velocity_review_24h: int = 20
    velocity_review_1h: int = 10
    score_block_threshold: float = 0.8
    score_review_threshold: float = 0.5

    # ── Event publisher seam (real Kafka/SQS transport deferred to work15). The durable outbox is
    # the compliance.compliance_events table, written in-transaction; this only selects the live
    # seam. ──
    event_publisher_mode: Literal["log", "noop"] = "log"

    # ── Idempotency ──
    idempotency_ttl_seconds: int = 24 * 60 * 60

    def callback_secrets_map(self) -> dict[str, str]:
        """Parse COMPLIANCE_CALLBACK_SECRETS into ``{provider: secret}``."""
        secrets: dict[str, str] = {}
        for entry in self.callback_secrets.split(";"):
            entry = entry.strip()
            if not entry or ":" not in entry:
                continue
            provider, _, raw = entry.partition(":")
            provider = provider.strip()
            secret = raw.strip()
            if provider and secret:
                secrets[provider] = secret
        return secrets

    def risk_config(self) -> RiskConfig:
        """Build the immutable :class:`RiskConfig` the pure risk engine consumes."""
        from app.domain.risk_engine import RiskConfig

        tier2 = self.tier2_ceiling.strip()
        tier2_ceiling = Decimal(tier2) if tier2 else Decimal("Infinity")
        return RiskConfig(
            tier_ceilings={0: Decimal(0), 1: self.tier1_ceiling, 2: tier2_ceiling},
            aml_cumulative_threshold=self.aml_cumulative_threshold,
            velocity_block_24h=self.velocity_block_24h,
            velocity_review_24h=self.velocity_review_24h,
            velocity_review_1h=self.velocity_review_1h,
            score_block_threshold=self.score_block_threshold,
            score_review_threshold=self.score_review_threshold,
        )


@lru_cache
def get_settings() -> Settings:
    """Cached settings accessor (one instance per process)."""
    return Settings()
