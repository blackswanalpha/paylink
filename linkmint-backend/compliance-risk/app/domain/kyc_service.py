"""KYC tier orchestration: start a verification session, apply a provider callback, read status.

``create_session`` validates the requested tier, refuses if the user already holds an equal/higher
tier (409 ALREADY_VERIFIED), calls the provider's ``start()``, and upserts the ``kyc_records`` row
(provider + encrypted provider_ref; the tier is NOT changed until a passing callback). It is
idempotent at the route via ``Idempotency-Key``.

``apply_callback`` is reached only AFTER the route has HMAC-verified the raw body. It parses the
vendor callback, REDACTS the metadata via :func:`app.redaction.redact` BEFORE any write/log/emit
(invariant A.1 — raw PII never persists), and on a pass sets tier/verified_at/expires_at and emits
``compliance.kyc.passed`` with the EXACT ``{"user_id", "tier"}`` payload identity-service's
``KycConsumer`` reads; on a fail it emits ``compliance.kyc.failed {"user_id"}``.

``get_status`` returns the status view, or 404 when nothing is known about the user (no kyc record
AND no flags AND no risk rows).
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Any

from app.config import Settings
from app.db.models import KycRecordRow
from app.db.repositories import ComplianceRepository
from app.domain.models import KycTier
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.providers.base import CallbackResult, KycProvider, StartResult, UpstreamError
from app.redaction import redact
from app.security.provider_crypto import ProviderCipher

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class StatusView:
    user_id: str
    kyc_tier: int
    risk_score: float | None
    flags: list[dict[str, Any]]


class KycService:
    def __init__(
        self,
        repo: ComplianceRepository,
        commit: _Commit,
        publisher: Publisher,
        cipher: ProviderCipher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._cipher = cipher
        self._settings = settings

    async def create_session(
        self,
        *,
        provider: KycProvider,
        user_id: uuid.UUID,
        tier_requested: int,
    ) -> StartResult:
        if tier_requested not in (KycTier.BASIC, KycTier.ENHANCED):
            raise AppError(
                ErrorCode.INVALID_TIER,
                "tier_requested must be 1 (basic) or 2 (enhanced)",
                details={"tier_requested": tier_requested},
            )
        existing = await self._repo.get_kyc_record(user_id)
        current_tier = int(existing.tier) if existing is not None else 0
        if current_tier >= tier_requested:
            raise AppError(
                ErrorCode.ALREADY_VERIFIED,
                "user already holds an equal or higher KYC tier",
                details={"current_tier": current_tier, "tier_requested": tier_requested},
            )

        try:
            started = await provider.start(user_id, tier_requested)
        except UpstreamError as exc:
            raise AppError(
                ErrorCode.PROVIDER_ERROR,
                "KYC provider could not start a session",
                details={"provider": provider.name},
            ) from exc

        # Upsert the record with the provider + encrypted provider_ref. The TIER is left unchanged
        # (only a passing callback grants a tier). provider_ref is encrypted at rest (KMS stand-in).
        await self._repo.upsert_kyc_record(
            KycRecordRow(
                user_id=user_id,
                tier=current_tier,
                provider=provider.name,
                provider_ref=self._cipher.encrypt(started.session_id),
                documents=existing.documents if existing is not None else None,
                verified_at=existing.verified_at if existing is not None else None,
                expires_at=existing.expires_at if existing is not None else None,
            )
        )
        await self._commit()
        return started

    async def apply_callback(self, *, provider: KycProvider, body: dict[str, Any]) -> None:
        result: CallbackResult = provider.parse_callback(body)
        # REDACT the vendor metadata BEFORE any write/log/emit — raw PII never persists.
        redacted = redact(result.metadata)
        existing = await self._repo.get_kyc_record(result.user_id)
        current_tier = int(existing.tier) if existing is not None else 0

        if result.passed:
            now = datetime.now(UTC)
            new_tier = max(current_tier, result.tier_granted)
            expires_at = now + timedelta(days=self._settings.kyc_validity_days)
            await self._repo.upsert_kyc_record(
                KycRecordRow(
                    user_id=result.user_id,
                    tier=new_tier,
                    provider=provider.name,
                    provider_ref=self._cipher.encrypt(result.provider_ref),
                    documents=redacted,  # redacted metadata ONLY
                    verified_at=now,
                    expires_at=expires_at,
                )
            )
            payload: dict[str, Any] = {"user_id": str(result.user_id), "tier": new_tier}
            await self._repo.add_event("user", result.user_id, events.KYC_PASSED, payload)
            await self._commit()
            await self._publisher.publish(events.KYC_PASSED, payload)
        else:
            await self._repo.upsert_kyc_record(
                KycRecordRow(
                    user_id=result.user_id,
                    tier=current_tier,
                    provider=provider.name,
                    provider_ref=(
                        self._cipher.encrypt(result.provider_ref)
                        if result.provider_ref
                        else (existing.provider_ref if existing is not None else None)
                    ),
                    documents=redacted or (existing.documents if existing is not None else None),
                    verified_at=existing.verified_at if existing is not None else None,
                    expires_at=existing.expires_at if existing is not None else None,
                )
            )
            fail_payload = {"user_id": str(result.user_id)}
            await self._repo.add_event("user", result.user_id, events.KYC_FAILED, fail_payload)
            await self._commit()
            await self._publisher.publish(events.KYC_FAILED, fail_payload)

    async def get_status(self, user_id: uuid.UUID) -> StatusView:
        record = await self._repo.get_kyc_record(user_id)
        latest = await self._repo.latest_risk_score(user_id)
        flags = await self._repo.list_open_flags(user_id)
        flag_count = await self._repo.count_flags(user_id)

        if record is None and flag_count == 0 and latest is None:
            raise AppError(ErrorCode.COMPLIANCE_NOT_FOUND, "no compliance record for this user")

        return StatusView(
            user_id=str(user_id),
            kyc_tier=int(record.tier) if record is not None else 0,
            risk_score=float(latest.score) if latest is not None else None,
            flags=[
                {
                    "kind": f.kind,
                    "severity": f.severity,
                    "raised_at": f.raised_at.isoformat() if f.raised_at is not None else None,
                }
                for f in flags
            ],
        )
