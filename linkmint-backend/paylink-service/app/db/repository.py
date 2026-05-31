"""Data access for PayLinks. Owns cursor pagination over (created_at, pl_id)."""

from __future__ import annotations

import base64
from datetime import datetime
from typing import Any

from sqlalchemy import select, tuple_
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import PayLinkEventRow, PayLinkRow


def encode_cursor(created_at: datetime, pl_id: str) -> str:
    raw = f"{created_at.isoformat()}|{pl_id}".encode()
    return base64.urlsafe_b64encode(raw).decode()


def decode_cursor(cursor: str) -> tuple[datetime, str]:
    raw = base64.urlsafe_b64decode(cursor.encode()).decode()
    ts_str, pl_id = raw.split("|", 1)
    return datetime.fromisoformat(ts_str), pl_id


class PayLinkRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def insert(self, row: PayLinkRow) -> PayLinkRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get(self, pl_id: str) -> PayLinkRow | None:
        return await self._session.get(PayLinkRow, pl_id)

    async def add_event(self, pl_id: str, kind: str, payload: dict[str, Any]) -> None:
        self._session.add(PayLinkEventRow(pl_id=pl_id, kind=kind, payload=payload))
        await self._session.flush()

    async def list_paylinks(
        self,
        *,
        creator: str | None = None,
        receiver: str | None = None,
        status: str | None = None,
        limit: int = 20,
        cursor: str | None = None,
    ) -> tuple[list[PayLinkRow], str | None]:
        stmt = select(PayLinkRow)
        if creator is not None:
            stmt = stmt.where(PayLinkRow.creator_addr == creator)
        if receiver is not None:
            stmt = stmt.where(PayLinkRow.receiver_addr == receiver)
        if status is not None:
            stmt = stmt.where(PayLinkRow.status == status)
        if cursor is not None:
            c_ts, c_id = decode_cursor(cursor)
            stmt = stmt.where(tuple_(PayLinkRow.created_at, PayLinkRow.pl_id) < (c_ts, c_id))
        stmt = stmt.order_by(PayLinkRow.created_at.desc(), PayLinkRow.pl_id.desc()).limit(limit + 1)

        rows = list((await self._session.execute(stmt)).scalars().all())
        next_cursor: str | None = None
        if len(rows) > limit:
            rows = rows[:limit]
            last = rows[-1]
            next_cursor = encode_cursor(last.created_at, last.pl_id)
        return rows, next_cursor
