"""Data access for the ``compliance`` schema.

One session-bound repository exposes every query the domain services need. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate a fetched row and rely on the
service's ``commit``. Tests substitute an in-memory fake with the same surface.

``activity_events`` is the hot path: windowed velocity counts and the cumulative-amount AML sum are
single indexed scans on ``(user_id, occurred_at)``.
"""

from __future__ import annotations

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    ActivityEventRow,
    ComplianceEventRow,
    FlagRow,
    KycRecordRow,
    RiskScoreRow,
)


class ComplianceRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── kyc records ──
    async def get_kyc_record(self, user_id: uuid.UUID) -> KycRecordRow | None:
        return await self._session.get(KycRecordRow, user_id)

    async def upsert_kyc_record(self, row: KycRecordRow) -> KycRecordRow:
        merged = await self._session.merge(row)
        await self._session.flush()
        return merged

    async def get_tier(self, user_id: uuid.UUID) -> int:
        """The user's current KYC tier (0 when there is no record)."""
        record = await self._session.get(KycRecordRow, user_id)
        return int(record.tier) if record is not None else 0

    # ── risk scores ──
    async def insert_risk_score(self, row: RiskScoreRow) -> RiskScoreRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def latest_risk_score(self, user_id: uuid.UUID) -> RiskScoreRow | None:
        stmt = (
            select(RiskScoreRow)
            .where(RiskScoreRow.user_id == user_id)
            .order_by(RiskScoreRow.evaluated_at.desc(), RiskScoreRow.id.desc())
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalar_one_or_none()

    # ── flags ──
    async def insert_flag(self, row: FlagRow) -> FlagRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def list_open_flags(self, user_id: uuid.UUID) -> list[FlagRow]:
        stmt = (
            select(FlagRow)
            .where(FlagRow.user_id == user_id, FlagRow.resolved_at.is_(None))
            .order_by(FlagRow.raised_at.desc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def count_flags(self, user_id: uuid.UUID) -> int:
        stmt = select(func.count()).select_from(FlagRow).where(FlagRow.user_id == user_id)
        return int((await self._session.execute(stmt)).scalar_one())

    # ── activity events (velocity / AML windows) ──
    async def insert_activity(self, row: ActivityEventRow) -> ActivityEventRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def count_activity_since(self, user_id: uuid.UUID, since: datetime) -> int:
        stmt = (
            select(func.count())
            .select_from(ActivityEventRow)
            .where(
                ActivityEventRow.user_id == user_id,
                ActivityEventRow.occurred_at >= since,
            )
        )
        return int((await self._session.execute(stmt)).scalar_one())

    async def sum_amount_since(self, user_id: uuid.UUID, since: datetime) -> Decimal:
        stmt = select(func.coalesce(func.sum(ActivityEventRow.amount), 0)).where(
            ActivityEventRow.user_id == user_id,
            ActivityEventRow.occurred_at >= since,
        )
        total = (await self._session.execute(stmt)).scalar_one()
        return Decimal(total) if total is not None else Decimal(0)

    # ── events (outbox) ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self._session.add(
            ComplianceEventRow(
                subject_type=subject_type, subject_id=subject_id, kind=kind, payload=payload
            )
        )
        await self._session.flush()
