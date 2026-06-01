"""User profile reads/updates + the KYC-tier mutation the compliance consumer drives."""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable

from app.db.models import UserRow
from app.db.repositories import IdentityRepository
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher

_Commit = Callable[[], Awaitable[None]]


class UsersService:
    def __init__(self, repo: IdentityRepository, commit: _Commit, publisher: Publisher) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher

    async def get(self, user_id: uuid.UUID) -> UserRow:
        user = await self._repo.get_user(user_id)
        if user is None:
            raise AppError(ErrorCode.USER_NOT_FOUND, "user not found")
        return user

    async def roles(self, user_id: uuid.UUID) -> list[tuple[str, str]]:
        """Fresh org-scoped roles ``(org_id, role)`` — used by /users/me (not the stale token)."""
        memberships = await self._repo.list_memberships_for_user(user_id)
        return [(str(m.org_id), m.role) for m in memberships]

    async def search(self, q: str, limit: int = 20) -> list[UserRow]:
        """Admin search by email/phone substring or exact user_id (internal-only surface)."""
        return await self._repo.search_users(q, limit)

    async def update(self, user_id: uuid.UUID, *, email: str | None, phone: str | None) -> UserRow:
        user = await self.get(user_id)
        if email is not None and email != user.email:
            if await self._repo.get_user_by_email(email):
                raise AppError(ErrorCode.EMAIL_TAKEN, "email already registered")
            user.email = email
        if phone is not None and phone != user.phone:
            if await self._repo.get_user_by_phone(phone):
                raise AppError(ErrorCode.PHONE_TAKEN, "phone already registered")
            user.phone = phone
        await self._commit()
        return user

    async def set_kyc_tier(self, user_id: uuid.UUID, tier: int) -> None:
        """Applied by the compliance.kyc.* consumer seam (work16). No-op if the user is unknown."""
        user = await self._repo.get_user(user_id)
        if user is None:
            return
        user.kyc_tier = tier
        await self._commit()
        await self._publisher.publish(
            events.USER_VERIFIED, {"user_id": str(user_id), "kyc_tier": tier}
        )
