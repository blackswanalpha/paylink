"""Data access for the ``refund`` schema.

One session-bound repository exposes every query the domain services need. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate fetched rows and rely on the
service's ``commit``. The dispute intake uses Postgres ``ON CONFLICT`` for webhook anti-replay.
Tests substitute an in-memory fake with the same surface.
"""

from __future__ import annotations

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import func, select
from sqlalchemy.dialects.postgresql import insert as pg_insert
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    DisputeEvidenceRow,
    DisputeRow,
    RefundEventRow,
    RefundRow,
    VerifiedPaylinkRow,
)

# Refund states that still count toward the cumulative-refund cap (not rejected/failed).
_ACTIVE_REFUND_STATES = ("REQUESTED", "APPROVED", "PROCESSING", "COMPLETED")


class RefundRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── refunds ──
    async def insert_refund(self, row: RefundRow) -> RefundRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_refund(self, refund_id: uuid.UUID) -> RefundRow | None:
        return await self._session.get(RefundRow, refund_id)

    async def list_refunds_by_payment(self, payment_id: str) -> list[RefundRow]:
        stmt = (
            select(RefundRow)
            .where(RefundRow.payment_id == payment_id)
            .order_by(RefundRow.created_at.asc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def active_refund_total(self, payment_id: str) -> int:
        """Sum of amounts for non-rejected/non-failed refunds of ``payment_id`` (the cap base)."""
        stmt = select(func.coalesce(func.sum(RefundRow.amount_minor), 0)).where(
            RefundRow.payment_id == payment_id,
            RefundRow.state.in_(_ACTIVE_REFUND_STATES),
        )
        return int((await self._session.execute(stmt)).scalar_one())

    async def list_processing_refunds_before(self, cutoff: datetime) -> list[RefundRow]:
        """PROCESSING refunds last touched before ``cutoff`` — the dev simulate-completion sweep."""
        stmt = select(RefundRow).where(
            RefundRow.state == "PROCESSING",
            RefundRow.updated_at < cutoff,
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── disputes ──
    async def insert_dispute_if_absent(self, row: DisputeRow) -> bool:
        """Insert a dispute, skipping on a duplicate ``(provider, provider_dispute_id)``. Returns
        True when a new row was inserted (anti-replay: a replayed open is a no-op)."""
        stmt = (
            pg_insert(DisputeRow)
            .values(
                dispute_id=row.dispute_id,
                provider=row.provider,
                provider_dispute_id=row.provider_dispute_id,
                payment_id=row.payment_id,
                paylink_id=row.paylink_id,
                rail=row.rail,
                merchant_id=row.merchant_id,
                org_id=row.org_id,
                amount_minor=row.amount_minor,
                currency=row.currency,
                reason_code=row.reason_code,
                state=row.state,
                evidence_due_at=row.evidence_due_at,
            )
            .on_conflict_do_nothing(index_elements=["provider", "provider_dispute_id"])
            .returning(DisputeRow.dispute_id)
        )
        inserted = (await self._session.execute(stmt)).scalar_one_or_none()
        await self._session.flush()
        return inserted is not None

    async def get_dispute(self, dispute_id: uuid.UUID) -> DisputeRow | None:
        return await self._session.get(DisputeRow, dispute_id)

    async def get_dispute_by_provider_ref(
        self, provider: str, provider_dispute_id: str
    ) -> DisputeRow | None:
        stmt = select(DisputeRow).where(
            DisputeRow.provider == provider,
            DisputeRow.provider_dispute_id == provider_dispute_id,
        )
        return (await self._session.execute(stmt)).scalars().first()

    async def list_open_disputes_due_before(self, cutoff: datetime) -> list[DisputeRow]:
        stmt = select(DisputeRow).where(
            DisputeRow.state == "OPEN",
            DisputeRow.evidence_due_at < cutoff,
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── dispute evidence ──
    async def insert_evidence(self, row: DisputeEvidenceRow) -> DisputeEvidenceRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def list_evidence(self, dispute_id: uuid.UUID) -> list[DisputeEvidenceRow]:
        stmt = (
            select(DisputeEvidenceRow)
            .where(DisputeEvidenceRow.dispute_id == dispute_id)
            .order_by(DisputeEvidenceRow.created_at.asc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def count_evidence(self, dispute_id: uuid.UUID) -> int:
        stmt = select(func.count()).where(DisputeEvidenceRow.dispute_id == dispute_id)
        return int((await self._session.execute(stmt)).scalar_one())

    # ── verified-paylink projection ──
    async def upsert_verified_paylink(
        self,
        *,
        paylink_id: str,
        tx_hash: str | None,
        block_height: int | None,
        amount_minor: int | None,
        currency: str | None,
        verified_at: datetime,
        payload: dict[str, Any],
    ) -> None:
        values: dict[str, Any] = {
            "paylink_id": paylink_id,
            "tx_hash": tx_hash,
            "block_height": block_height,
            "amount_minor": Decimal(amount_minor) if amount_minor is not None else None,
            "currency": currency,
            "verified_at": verified_at,
            "payload": payload,
        }
        stmt = pg_insert(VerifiedPaylinkRow).values(**values)
        stmt = stmt.on_conflict_do_update(
            index_elements=[VerifiedPaylinkRow.paylink_id],
            set_={
                "tx_hash": stmt.excluded.tx_hash,
                "block_height": stmt.excluded.block_height,
                # Never clear a known amount with a later event that omits it.
                "amount_minor": func.coalesce(
                    stmt.excluded.amount_minor, VerifiedPaylinkRow.amount_minor
                ),
                "currency": func.coalesce(stmt.excluded.currency, VerifiedPaylinkRow.currency),
                "verified_at": stmt.excluded.verified_at,
                "payload": stmt.excluded.payload,
            },
        )
        await self._session.execute(stmt)
        await self._session.flush()

    async def get_verified_paylink(self, paylink_id: str) -> VerifiedPaylinkRow | None:
        return await self._session.get(VerifiedPaylinkRow, paylink_id)

    # ── events (outbox) ──
    async def add_event(self, entity_id: str, kind: str, payload: dict[str, Any]) -> None:
        self._session.add(RefundEventRow(entity_id=entity_id, kind=kind, payload=payload))
        await self._session.flush()
