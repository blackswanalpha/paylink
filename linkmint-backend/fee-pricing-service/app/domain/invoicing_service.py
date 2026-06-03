"""Platform-fee invoicing: accrual intake + monthly invoice generation.

Accruals are realized platform fees (recorded via the trusted-network ``/v1/internal/accruals`` or
the optional event seam), idempotent on ``(merchant_id, source_ref)``. Monthly generation rolls a
period's unbilled accruals into one invoice per merchant (idempotent on ``(merchant_id, period)``),
stamps the accruals, posts to the ledger seam (A.6, no-op by default), and emits
``invoice.platform_fee.issued``. The internal endpoint and the sweeper both call generation.
"""

from __future__ import annotations

import re
import uuid
from collections import defaultdict
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import Decimal

from app.config import Settings
from app.db.models import PlatformFeeAccrualRow, PlatformFeeInvoiceRow
from app.db.repositories import PricingRepository
from app.errors import AppError, ErrorCode
from app.events.publisher import INVOICE_PLATFORM_FEE_ISSUED, Publisher
from app.ledger.poster import LedgerPoster
from app.logging import get_logger

log = get_logger("pricing.invoicing")

_Commit = Callable[[], Awaitable[None]]
_PERIOD_RE = re.compile(r"^\d{4}-(0[1-9]|1[0-2])$")


def period_of(occurred_at: datetime) -> str:
    """The 'YYYY-MM' billing period (UTC) an event falls in."""
    if occurred_at.tzinfo is None:
        occurred_at = occurred_at.replace(tzinfo=UTC)
    return occurred_at.astimezone(UTC).strftime("%Y-%m")


@dataclass(frozen=True)
class AccrualResult:
    accrual_id: int
    inserted: bool


@dataclass(frozen=True)
class GeneratedInvoice:
    invoice_id: str
    merchant_id: str
    currency: str
    total_fee: int
    line_count: int


@dataclass(frozen=True)
class GenerationResult:
    period: str
    generated: list[GeneratedInvoice]
    skipped_existing: int


class InvoicingService:
    def __init__(
        self,
        repo: PricingRepository,
        commit: _Commit,
        publisher: Publisher,
        ledger: LedgerPoster,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._ledger = ledger
        self._settings = settings

    async def record_accrual(
        self,
        *,
        merchant_id: uuid.UUID,
        amount: int,
        currency: str,
        source_ref: str,
        occurred_at: datetime,
        quote_id: uuid.UUID | None = None,
    ) -> AccrualResult:
        """Idempotent on ``(merchant_id, source_ref)`` — a duplicate returns the existing row."""
        inserted, accrual_id = await self._repo.insert_accrual(
            merchant_id=merchant_id,
            period=period_of(occurred_at),
            amount=Decimal(amount),
            currency=currency.upper(),
            source_ref=source_ref,
            occurred_at=occurred_at,
            quote_id=quote_id,
        )
        await self._commit()
        log.info(
            "accrual_recorded",
            merchant_id=str(merchant_id),
            source_ref=source_ref,
            inserted=inserted,
        )
        return AccrualResult(accrual_id=accrual_id, inserted=inserted)

    async def generate_for_period(
        self, period: str, *, merchant_id: uuid.UUID | None = None
    ) -> GenerationResult:
        """Aggregate a period's unbilled accruals into one invoice per merchant. Idempotent: a
        re-run skips merchants that already have an invoice for the period."""
        if not _PERIOD_RE.match(period):
            raise AppError(
                ErrorCode.INVALID_PERIOD, "period must be YYYY-MM", details={"period": period}
            )

        rows = await self._repo.unbilled_accruals(period, merchant_id=merchant_id)
        groups: dict[tuple[uuid.UUID, str], list[PlatformFeeAccrualRow]] = defaultdict(list)
        for r in rows:
            groups[(r.merchant_id, r.currency)].append(r)

        generated: list[GeneratedInvoice] = []
        skipped = 0
        for (mid, currency), grp in groups.items():
            existing = await self._repo.get_invoice_for_period(mid, period)
            if existing is not None:
                # Idempotent re-run, or a prior currency-group already billed this merchant+period.
                skipped += 1
                continue
            total = sum((r.amount for r in grp), Decimal(0))
            invoice_id = uuid.uuid4()
            await self._repo.insert_invoice(
                PlatformFeeInvoiceRow(
                    invoice_id=invoice_id,
                    merchant_id=mid,
                    period=period,
                    currency=currency,
                    total_fee=total,
                    line_count=len(grp),
                    status="ISSUED",
                )
            )
            await self._repo.mark_accruals_invoiced([r.id for r in grp], invoice_id)
            if self._settings.ledger_posting_enabled:
                await self._ledger.post_platform_fee(
                    invoice_id=invoice_id,
                    merchant_id=mid,
                    period=period,
                    currency=currency,
                    total_fee=total,
                )
            payload = {
                "invoice_id": str(invoice_id),
                "merchant_id": str(mid),
                "period": period,
                "currency": currency,
                "total_fee": int(total),
                "line_count": len(grp),
            }
            await self._repo.add_event(str(invoice_id), INVOICE_PLATFORM_FEE_ISSUED, payload)
            await self._publisher.publish(INVOICE_PLATFORM_FEE_ISSUED, payload)
            generated.append(
                GeneratedInvoice(
                    invoice_id=str(invoice_id),
                    merchant_id=str(mid),
                    currency=currency,
                    total_fee=int(total),
                    line_count=len(grp),
                )
            )
        await self._commit()
        log.info(
            "platform_fee_invoices_generated",
            period=period,
            generated=len(generated),
            skipped=skipped,
        )
        return GenerationResult(period=period, generated=generated, skipped_existing=skipped)
