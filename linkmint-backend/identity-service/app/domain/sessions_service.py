"""Sessions + token issuance: login/oauth issue, refresh rotation (with reuse-detection), logout,
list, and per-session revoke.

Refresh tokens rotate single-use: each ``/refresh`` revokes the presented session and mints a new
one. Presenting an already-rotated (revoked) refresh token is treated as theft — the whole session
family for that user is revoked and ``identity.auth.failed`` is emitted.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime, timedelta

from app.config import Settings
from app.db.models import SessionRow, UserRow
from app.db.repositories import IdentityRepository
from app.domain.models import AuthTokens, UserRole, UserStatus
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.jwt import JwtIssuer, OrgRole
from app.security.refresh_tokens import hash_refresh_token, mint_refresh_token

_Commit = Callable[[], Awaitable[None]]


def _aware(dt: datetime) -> datetime:
    return dt if dt.tzinfo is not None else dt.replace(tzinfo=UTC)


class SessionsService:
    def __init__(
        self,
        repo: IdentityRepository,
        commit: _Commit,
        jwt: JwtIssuer,
        publisher: Publisher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._jwt = jwt
        self._publisher = publisher
        self._settings = settings

    async def _roles(self, user_id: uuid.UUID) -> tuple[list[OrgRole], list[str]]:
        memberships = await self._repo.list_memberships_for_user(user_id)
        roles = [OrgRole(org_id=str(m.org_id), role=m.role) for m in memberships]
        return roles, [UserRole.PAYER.value]

    async def issue(
        self, user: UserRow, *, user_agent: str | None = None, ip: str | None = None
    ) -> AuthTokens:
        """Create a session + mint access/refresh for ``user``. Commits all pending writes."""
        refresh = mint_refresh_token()
        sid = uuid.uuid4()
        session = SessionRow(
            session_id=sid,
            user_id=user.user_id,
            refresh_token=hash_refresh_token(refresh),
            user_agent=user_agent,
            ip=ip,
            expires_at=datetime.now(UTC)
            + timedelta(seconds=self._settings.refresh_token_ttl_seconds),
        )
        await self._repo.insert_session(session)
        roles, user_roles = await self._roles(user.user_id)
        access, expires_in = self._jwt.issue_access(
            user_id=str(user.user_id),
            roles=roles,
            user_roles=user_roles,
            kyc_tier=user.kyc_tier,
            sid=str(sid),
        )
        await self._commit()
        return AuthTokens(access_token=access, refresh_token=refresh, expires_in=expires_in)

    async def rotate(
        self, refresh_token: str, *, user_agent: str | None = None, ip: str | None = None
    ) -> AuthTokens:
        session = await self._repo.get_session_by_refresh_hash(hash_refresh_token(refresh_token))
        if session is None:
            raise AppError(ErrorCode.INVALID_TOKEN, "invalid refresh token")
        now = datetime.now(UTC)
        if session.revoked_at is not None:
            # Reuse of a rotated-away token → likely theft. Revoke the whole family.
            await self._revoke_family(session.user_id)
            await self._commit()
            await self._publisher.publish(
                events.AUTH_FAILED,
                {"user_id": str(session.user_id), "reason": "refresh_token_reuse"},
            )
            raise AppError(ErrorCode.INVALID_TOKEN, "refresh token reuse detected")
        if _aware(session.expires_at) <= now:
            raise AppError(ErrorCode.TOKEN_EXPIRED, "refresh token expired")
        user = await self._repo.get_user(session.user_id)
        if user is None or user.status != UserStatus.ACTIVE:
            raise AppError(ErrorCode.INVALID_TOKEN, "invalid refresh token")
        session.revoked_at = now  # single-use: retire the presented session
        return await self.issue(user, user_agent=user_agent, ip=ip)  # issue() commits both writes

    async def logout(self, user_id: uuid.UUID, refresh_token: str) -> None:
        """Revoke the session behind a refresh token. Idempotent (no error if already gone)."""
        session = await self._repo.get_session_by_refresh_hash(hash_refresh_token(refresh_token))
        if session is not None and session.user_id == user_id and session.revoked_at is None:
            session.revoked_at = datetime.now(UTC)
            await self._commit()

    async def list_active(self, user_id: uuid.UUID) -> list[SessionRow]:
        now = datetime.now(UTC)
        sessions = await self._repo.list_sessions_for_user(user_id)
        return [s for s in sessions if s.revoked_at is None and _aware(s.expires_at) > now]

    async def revoke(self, user_id: uuid.UUID, session_id: uuid.UUID) -> None:
        session = await self._repo.get_session(session_id)
        if session is None or session.user_id != user_id:
            raise AppError(ErrorCode.SESSION_NOT_FOUND, "session not found")
        if session.revoked_at is None:
            session.revoked_at = datetime.now(UTC)
            await self._commit()

    async def _revoke_family(self, user_id: uuid.UUID) -> None:
        now = datetime.now(UTC)
        for s in await self._repo.list_active_sessions_for_user(user_id):
            s.revoked_at = now
