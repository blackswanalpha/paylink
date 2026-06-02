"""Data access for the ``invoice`` schema.

One session-bound repository exposes every query the domain service needs. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate a fetched row and rely on the
service's ``commit``. Tests substitute an in-memory fake with the same surface.
"""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import InvoiceEventRow, InvoiceLineRow, InvoiceRow


class InvoiceRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── invoices ──
    async def insert_invoice(self, row: InvoiceRow) -> InvoiceRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_invoice(self, invoice_id: uuid.UUID) -> InvoiceRow | None:
        return await self._session.get(InvoiceRow, invoice_id)

    async def get_by_plid(self, pl_id: str) -> InvoiceRow | None:
        stmt = select(InvoiceRow).where(InvoiceRow.pl_id == pl_id)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    async def list_invoices(
        self,
        merchant_id: uuid.UUID,
        *,
        status: str | None = None,
        limit: int = 50,
        offset: int = 0,
    ) -> list[InvoiceRow]:
        stmt = select(InvoiceRow).where(InvoiceRow.merchant_id == merchant_id)
        if status is not None:
            stmt = stmt.where(InvoiceRow.status == status)
        stmt = stmt.order_by(InvoiceRow.created_at.desc()).limit(limit).offset(offset)
        return list((await self._session.execute(stmt)).scalars().all())

    async def find_overdue(self, now: datetime, *, limit: int = 100) -> list[InvoiceRow]:
        """OPEN invoices past their due date (the sweeper scan; indexed on (status, due_at))."""
        stmt = (
            select(InvoiceRow)
            .where(InvoiceRow.status == "OPEN", InvoiceRow.due_at < now)
            .order_by(InvoiceRow.due_at.asc())
            .limit(limit)
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── lines ──
    async def insert_lines(self, rows: list[InvoiceLineRow]) -> list[InvoiceLineRow]:
        self._session.add_all(rows)
        await self._session.flush()
        return rows

    async def list_lines(self, invoice_id: uuid.UUID) -> list[InvoiceLineRow]:
        stmt = (
            select(InvoiceLineRow)
            .where(InvoiceLineRow.invoice_id == invoice_id)
            .order_by(InvoiceLineRow.id.asc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    # ── events (outbox) ──
    async def add_event(self, invoice_id: uuid.UUID, kind: str, payload: dict[str, Any]) -> None:
        self._session.add(InvoiceEventRow(invoice_id=invoice_id, kind=kind, payload=payload))
        await self._session.flush()
