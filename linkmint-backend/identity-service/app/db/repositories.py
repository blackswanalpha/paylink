"""Data access for the ``identity`` schema.

One session-bound repository exposes every query the domain services need. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate a fetched row and rely on the
service's ``commit``; ``delete_*`` removes + flushes. Tests substitute an in-memory fake with the
same surface.
"""

from __future__ import annotations

import contextlib
import uuid
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import ColumnElement, func, or_, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    ApiKeyRow,
    IdentityEventRow,
    MembershipRow,
    MfaFactorRow,
    OAuthIdentityRow,
    OrganizationRow,
    PasswordResetTokenRow,
    SessionRow,
    UserRow,
)


def _escape_like(value: str) -> str:
    """Escape LIKE wildcards so user input matches literally (escape char = backslash).

    Without this a ``q`` of ``%``/``_`` is a live wildcard — ``%`` would match every row and force
    an unindexed full scan. The escaped pattern is paired with ``.ilike(..., escape="\\")``.
    """
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


class IdentityRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── users ──
    async def insert_user(self, row: UserRow) -> UserRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_user(self, user_id: uuid.UUID) -> UserRow | None:
        return await self._session.get(UserRow, user_id)

    async def get_user_by_email(self, email: str) -> UserRow | None:
        stmt = select(UserRow).where(UserRow.email == email)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    async def get_user_by_phone(self, phone: str) -> UserRow | None:
        stmt = select(UserRow).where(UserRow.phone == phone)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    async def search_users(self, q: str, limit: int = 20) -> list[UserRow]:
        """Admin lookup: match an email/phone substring or an exact user_id (internal-only)."""
        like = f"%{_escape_like(q)}%"
        conditions: list[ColumnElement[bool]] = [
            UserRow.email.ilike(like, escape="\\"),
            UserRow.phone.ilike(like, escape="\\"),
        ]
        with contextlib.suppress(ValueError):  # q not a UUID → substring match only
            conditions.append(UserRow.user_id == uuid.UUID(q))
        stmt = (
            select(UserRow).where(or_(*conditions)).order_by(UserRow.created_at.desc()).limit(limit)
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── oauth identities ──
    async def get_oauth_identity(self, provider: str, subject: str) -> OAuthIdentityRow | None:
        return await self._session.get(OAuthIdentityRow, (provider, subject))

    async def insert_oauth_identity(self, row: OAuthIdentityRow) -> OAuthIdentityRow:
        self._session.add(row)
        await self._session.flush()
        return row

    # ── mfa factors ──
    async def get_mfa_factor(self, user_id: uuid.UUID, kind: str) -> MfaFactorRow | None:
        return await self._session.get(MfaFactorRow, (user_id, kind))

    async def insert_mfa_factor(self, row: MfaFactorRow) -> MfaFactorRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def delete_mfa_factor(self, row: MfaFactorRow) -> None:
        await self._session.delete(row)
        await self._session.flush()

    async def list_active_mfa(self, user_id: uuid.UUID) -> list[MfaFactorRow]:
        stmt = select(MfaFactorRow).where(
            MfaFactorRow.user_id == user_id, MfaFactorRow.activated_at.is_not(None)
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── organizations ──
    async def insert_org(self, row: OrganizationRow) -> OrganizationRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_org(self, org_id: uuid.UUID) -> OrganizationRow | None:
        return await self._session.get(OrganizationRow, org_id)

    # ── memberships ──
    async def insert_membership(self, row: MembershipRow) -> MembershipRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_membership(self, org_id: uuid.UUID, user_id: uuid.UUID) -> MembershipRow | None:
        return await self._session.get(MembershipRow, (org_id, user_id))

    async def list_members(self, org_id: uuid.UUID) -> list[MembershipRow]:
        stmt = select(MembershipRow).where(MembershipRow.org_id == org_id)
        return list((await self._session.execute(stmt)).scalars().all())

    async def list_memberships_for_user(self, user_id: uuid.UUID) -> list[MembershipRow]:
        stmt = select(MembershipRow).where(MembershipRow.user_id == user_id)
        return list((await self._session.execute(stmt)).scalars().all())

    async def list_orgs_for_user(self, user_id: uuid.UUID) -> list[tuple[OrganizationRow, str]]:
        """The user's organizations (joined for name/type) + their role in each, newest first."""
        stmt = (
            select(OrganizationRow, MembershipRow.role)
            .join(MembershipRow, MembershipRow.org_id == OrganizationRow.org_id)
            .where(MembershipRow.user_id == user_id)
            .order_by(OrganizationRow.created_at.desc())
        )
        return [(org, role) for org, role in (await self._session.execute(stmt)).all()]

    async def count_owners(self, org_id: uuid.UUID) -> int:
        stmt = (
            select(func.count())
            .select_from(MembershipRow)
            .where(MembershipRow.org_id == org_id, MembershipRow.role == "owner")
        )
        return int((await self._session.execute(stmt)).scalar_one())

    async def delete_membership(self, row: MembershipRow) -> None:
        await self._session.delete(row)
        await self._session.flush()

    # ── api keys ──
    async def insert_api_key(self, row: ApiKeyRow) -> ApiKeyRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_api_key(self, api_key_id: uuid.UUID) -> ApiKeyRow | None:
        return await self._session.get(ApiKeyRow, api_key_id)

    async def list_api_keys_for_org(self, org_id: uuid.UUID) -> list[ApiKeyRow]:
        stmt = (
            select(ApiKeyRow)
            .where(ApiKeyRow.org_id == org_id)
            .order_by(ApiKeyRow.created_at.desc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def list_api_keys_by_prefix(self, prefix: str) -> list[ApiKeyRow]:
        stmt = select(ApiKeyRow).where(ApiKeyRow.prefix == prefix, ApiKeyRow.status == "ACTIVE")
        return list((await self._session.execute(stmt)).scalars().all())

    # ── sessions ──
    async def insert_session(self, row: SessionRow) -> SessionRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_session(self, session_id: uuid.UUID) -> SessionRow | None:
        return await self._session.get(SessionRow, session_id)

    async def get_session_by_refresh_hash(self, token_hash: str) -> SessionRow | None:
        stmt = select(SessionRow).where(SessionRow.refresh_token == token_hash)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    async def list_sessions_for_user(self, user_id: uuid.UUID) -> list[SessionRow]:
        stmt = (
            select(SessionRow)
            .where(SessionRow.user_id == user_id)
            .order_by(SessionRow.created_at.desc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def list_active_sessions_for_user(self, user_id: uuid.UUID) -> list[SessionRow]:
        stmt = select(SessionRow).where(
            SessionRow.user_id == user_id, SessionRow.revoked_at.is_(None)
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── password reset tokens ──
    async def insert_password_reset_token(
        self, row: PasswordResetTokenRow
    ) -> PasswordResetTokenRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_password_reset_by_hash(self, token_hash: str) -> PasswordResetTokenRow | None:
        stmt = select(PasswordResetTokenRow).where(PasswordResetTokenRow.token_hash == token_hash)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    async def invalidate_user_reset_tokens(self, user_id: uuid.UUID) -> None:
        """Stamp ``used_at`` on any outstanding (unused) tokens so only the latest can be live."""
        now = datetime.now(UTC)
        stmt = select(PasswordResetTokenRow).where(
            PasswordResetTokenRow.user_id == user_id,
            PasswordResetTokenRow.used_at.is_(None),
        )
        for row in (await self._session.execute(stmt)).scalars().all():
            row.used_at = now
        await self._session.flush()

    # ── events (outbox) ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self._session.add(
            IdentityEventRow(
                subject_type=subject_type, subject_id=subject_id, kind=kind, payload=payload
            )
        )
        await self._session.flush()
