"""TOTP MFA: enroll (provisioning, inactive) → verify (activate) → disable. Phase 1 = TOTP only.

The TOTP secret is encrypted at rest (AES-GCM, the KMS stand-in) and only decrypted transiently to
verify a code. ``identity.mfa.enabled`` fires on first activation.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime

from app.config import Settings
from app.db.models import MfaFactorRow
from app.db.repositories import IdentityRepository
from app.domain.models import MfaKind
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.mfa_crypto import MfaCipher
from app.security.totp import generate_totp_secret, provisioning_uri, verify_totp

_Commit = Callable[[], Awaitable[None]]


class MfaService:
    def __init__(
        self,
        repo: IdentityRepository,
        commit: _Commit,
        cipher: MfaCipher,
        publisher: Publisher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._cipher = cipher
        self._publisher = publisher
        self._settings = settings

    async def is_required(self, user_id: uuid.UUID) -> bool:
        return len(await self._repo.list_active_mfa(user_id)) > 0

    async def verify_login(self, user_id: uuid.UUID, code: str) -> bool:
        for factor in await self._repo.list_active_mfa(user_id):
            if factor.kind == MfaKind.TOTP and verify_totp(
                self._cipher.decrypt(factor.secret), code
            ):
                return True
        return False

    async def enroll(self, user_id: uuid.UUID, *, account_name: str) -> tuple[str, str]:
        """Begin TOTP enrollment. Returns ``(secret, otpauth_uri)``; not active until verified."""
        existing = await self._repo.get_mfa_factor(user_id, MfaKind.TOTP)
        if existing is not None and existing.activated_at is not None:
            raise AppError(ErrorCode.MFA_ALREADY_ENROLLED, "TOTP already enrolled")
        secret = generate_totp_secret()
        ciphertext = self._cipher.encrypt(secret)
        if existing is not None:
            existing.secret = ciphertext  # re-enroll over an unverified factor
            existing.activated_at = None
        else:
            await self._repo.insert_mfa_factor(
                MfaFactorRow(user_id=user_id, kind=MfaKind.TOTP, secret=ciphertext)
            )
        await self._commit()
        uri = provisioning_uri(secret, account_name=account_name, issuer=self._settings.jwt_issuer)
        return secret, uri

    async def verify(self, user_id: uuid.UUID, code: str) -> None:
        factor = await self._repo.get_mfa_factor(user_id, MfaKind.TOTP)
        if factor is None:
            raise AppError(ErrorCode.MFA_NOT_ENROLLED, "no TOTP enrollment in progress")
        if not verify_totp(self._cipher.decrypt(factor.secret), code):
            raise AppError(ErrorCode.MFA_INVALID, "invalid TOTP code")
        if factor.activated_at is None:
            factor.activated_at = datetime.now(UTC)
            await self._commit()
            await self._publisher.publish(events.MFA_ENABLED, {"user_id": str(user_id)})

    async def disable(self, user_id: uuid.UUID, code: str) -> None:
        factor = await self._repo.get_mfa_factor(user_id, MfaKind.TOTP)
        if factor is None:
            raise AppError(ErrorCode.MFA_NOT_ENROLLED, "TOTP not enrolled")
        if not verify_totp(self._cipher.decrypt(factor.secret), code):
            raise AppError(ErrorCode.MFA_INVALID, "invalid TOTP code")
        await self._repo.delete_mfa_factor(factor)
        await self._commit()
