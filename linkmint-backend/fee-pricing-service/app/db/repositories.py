"""Data access for the ``pricing`` schema.

One session-bound repository exposes every query the domain services need. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate fetched rows and rely on the
service's ``commit``. Idempotent intakes (accruals, merchant-pricing) use Postgres ``ON CONFLICT``.
Tests substitute an in-memory fake with the same surface.
"""

from __future__ import annotations

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import func, select, update
from sqlalchemy.dialects.postgresql import insert as pg_insert
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    FxRateRow,
    MerchantPricingRow,
    PlatformFeeAccrualRow,
    PlatformFeeInvoiceRow,
    PricingEventRow,
    QuoteRow,
    RailFeeRow,
    TierRow,
)


class PricingRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── tiers ──
    async def get_tier(self, tier: str) -> TierRow | None:
        return await self._session.get(TierRow, tier)

    async def list_tiers(self, *, active_only: bool = False) -> list[TierRow]:
        stmt = select(TierRow)
        if active_only:
            stmt = stmt.where(TierRow.active.is_(True))
        stmt = stmt.order_by(TierRow.platform_pct_bps.desc())
        return list((await self._session.execute(stmt)).scalars().all())

    # ── rail fee schedules ──
    async def get_rail_fee(self, rail: str, tier: str) -> RailFeeRow | None:
        """The effective active fee for ``rail``: a tier-specific override beats the global row."""
        stmt = (
            select(RailFeeRow)
            .where(
                RailFeeRow.rail == rail,
                RailFeeRow.active.is_(True),
                (RailFeeRow.tier == tier) | (RailFeeRow.tier.is_(None)),
            )
            # tier-specific (tier NOT NULL → is_(None)==False==0) sorts before the global row.
            .order_by(RailFeeRow.tier.is_(None))
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalars().first()

    async def list_rail_fees(self, *, active_only: bool = True) -> list[RailFeeRow]:
        stmt = select(RailFeeRow)
        if active_only:
            stmt = stmt.where(RailFeeRow.active.is_(True))
        stmt = stmt.order_by(RailFeeRow.rail.asc(), RailFeeRow.tier.is_(None))
        return list((await self._session.execute(stmt)).scalars().all())

    # ── merchant pricing ──
    async def get_merchant_pricing(self, merchant_id: uuid.UUID) -> MerchantPricingRow | None:
        return await self._session.get(MerchantPricingRow, merchant_id)

    async def upsert_merchant_pricing(
        self,
        merchant_id: uuid.UUID,
        *,
        tier: str,
        source: str,
        org_id: uuid.UUID | None = None,
    ) -> None:
        """Insert-or-update a merchant's tier. A null ``org_id`` never clears a previously captured
        one (``fee_tier.changed`` carries no org_id), so the read-gate mapping is preserved."""
        stmt = pg_insert(MerchantPricingRow).values(
            merchant_id=merchant_id,
            org_id=org_id,
            tier=tier,
            source=source,
        )
        stmt = stmt.on_conflict_do_update(
            index_elements=[MerchantPricingRow.merchant_id],
            set_={
                "tier": stmt.excluded.tier,
                "source": stmt.excluded.source,
                "org_id": func.coalesce(stmt.excluded.org_id, MerchantPricingRow.org_id),
                "effective_at": func.now(),
                "updated_at": func.now(),
            },
        )
        await self._session.execute(stmt)
        await self._session.flush()

    # ── quotes ──
    async def insert_quote(self, row: QuoteRow) -> QuoteRow:
        self._session.add(row)
        await self._session.flush()
        return row

    # ── fx rates ──
    async def insert_fx_rate(self, row: FxRateRow) -> FxRateRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def latest_fx_rate(self, base: str, quote: str) -> FxRateRow | None:
        stmt = (
            select(FxRateRow)
            .where(FxRateRow.base_currency == base, FxRateRow.quote_currency == quote)
            .order_by(FxRateRow.fetched_at.desc())
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalars().first()

    # ── platform-fee accruals ──
    async def insert_accrual(
        self,
        *,
        merchant_id: uuid.UUID,
        period: str,
        amount: Decimal,
        currency: str,
        source_ref: str,
        occurred_at: datetime,
        quote_id: uuid.UUID | None = None,
    ) -> tuple[bool, int]:
        """Idempotent intake: ``(inserted, accrual_id)``. A duplicate ``(merchant_id, source_ref)``
        is a no-op and returns the existing row's id."""
        stmt = (
            pg_insert(PlatformFeeAccrualRow)
            .values(
                merchant_id=merchant_id,
                period=period,
                amount=amount,
                currency=currency,
                source_ref=source_ref,
                occurred_at=occurred_at,
                quote_id=quote_id,
            )
            .on_conflict_do_nothing(index_elements=["merchant_id", "source_ref"])
            .returning(PlatformFeeAccrualRow.id)
        )
        new_id = (await self._session.execute(stmt)).scalar_one_or_none()
        if new_id is not None:
            await self._session.flush()
            return True, int(new_id)
        existing = await self._session.execute(
            select(PlatformFeeAccrualRow.id).where(
                PlatformFeeAccrualRow.merchant_id == merchant_id,
                PlatformFeeAccrualRow.source_ref == source_ref,
            )
        )
        return False, int(existing.scalar_one())

    async def unbilled_accruals(
        self, period: str, *, merchant_id: uuid.UUID | None = None
    ) -> list[PlatformFeeAccrualRow]:
        stmt = select(PlatformFeeAccrualRow).where(
            PlatformFeeAccrualRow.period == period,
            PlatformFeeAccrualRow.invoice_id.is_(None),
        )
        if merchant_id is not None:
            stmt = stmt.where(PlatformFeeAccrualRow.merchant_id == merchant_id)
        stmt = stmt.order_by(
            PlatformFeeAccrualRow.merchant_id.asc(), PlatformFeeAccrualRow.id.asc()
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def mark_accruals_invoiced(self, ids: list[int], invoice_id: uuid.UUID) -> None:
        if not ids:
            return
        await self._session.execute(
            update(PlatformFeeAccrualRow)
            .where(PlatformFeeAccrualRow.id.in_(ids))
            .values(invoice_id=invoice_id)
        )
        await self._session.flush()

    # ── platform-fee invoices ──
    async def get_invoice_for_period(
        self, merchant_id: uuid.UUID, period: str
    ) -> PlatformFeeInvoiceRow | None:
        stmt = select(PlatformFeeInvoiceRow).where(
            PlatformFeeInvoiceRow.merchant_id == merchant_id,
            PlatformFeeInvoiceRow.period == period,
        )
        return (await self._session.execute(stmt)).scalars().first()

    async def insert_invoice(self, row: PlatformFeeInvoiceRow) -> PlatformFeeInvoiceRow:
        self._session.add(row)
        await self._session.flush()
        return row

    # ── events (outbox) ──
    async def add_event(self, entity_id: str, kind: str, payload: dict[str, Any]) -> None:
        self._session.add(PricingEventRow(entity_id=entity_id, kind=kind, payload=payload))
        await self._session.flush()
