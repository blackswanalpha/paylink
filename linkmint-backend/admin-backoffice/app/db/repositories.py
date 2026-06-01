"""Data access for the ``admin`` schema (Phase 1: staff scope grants only)."""

from __future__ import annotations

import uuid

from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import StaffRow


class AdminRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def get_staff(self, sub: str) -> StaffRow | None:
        """The staff grant row for a JWT ``sub``, or None (default-deny) if ``sub`` is not staff."""
        try:
            sid = uuid.UUID(sub)
        except ValueError:
            return None
        return await self._session.get(StaffRow, sid)
