"""Invoice domain service + the per-request service bundle.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
the bus consumer and sweeper build the same bundle over a fresh session, so there is a single place
the lifecycle rules live. Tests build the bundle over in-memory fakes (same surface).

Lifecycle: DRAFT → OPEN → PAID | VOID | OVERDUE. Finalize is one-way (only DRAFT). Void is blocked
once PAID. Settlement truth comes from the chain (``mark_paid_by_plid`` is driven by the
``chain.paylink.verified`` consumer), never set by the merchant directly (non-custodial invariant).
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any, Protocol

from app.config import Settings
from app.db.models import InvoiceLineRow, InvoiceRow
from app.db.repositories import InvoiceRepository
from app.domain.models import InvoiceStatus, LineInput, compute_totals
from app.errors import AppError, ErrorCode
from app.events.publisher import (
    INVOICE_CREATED,
    INVOICE_FINALIZED,
    INVOICE_OVERDUE,
    INVOICE_PAID,
    INVOICE_VOIDED,
    Publisher,
)
from app.logging import get_logger
from app.paylinks.client import PaylinkError

log = get_logger("invoice.service")

_Commit = Callable[[], Awaitable[None]]


class PaylinkPort(Protocol):
    """The slice of paylink-service the invoice domain depends on (real client or a test fake)."""

    async def create(
        self,
        *,
        receiver: str,
        amount: int,
        currency: str,
        expiry: datetime,
        usage: str = "single",
        metadata: dict[str, Any] | None = None,
        idempotency_key: str | None = None,
    ) -> dict[str, Any]: ...


@dataclass(frozen=True)
class ServiceDeps:
    repo: InvoiceRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    paylink: PaylinkPort


@dataclass(frozen=True)
class Services:
    invoices: InvoiceService


def build_services(d: ServiceDeps) -> Services:
    return Services(invoices=InvoiceService(d.repo, d.commit, d.publisher, d.paylink, d.settings))


class InvoiceService:
    def __init__(
        self,
        repo: InvoiceRepository,
        commit: _Commit,
        publisher: Publisher,
        paylink: PaylinkPort,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._paylink = paylink
        self._settings = settings

    async def _emit(self, invoice_id: uuid.UUID, kind: str, payload: dict[str, Any]) -> None:
        """Write the durable outbox row + the in-process echo (both in the caller's transaction)."""
        await self._repo.add_event(invoice_id, kind, payload)
        await self._publisher.publish(kind, payload)

    async def _load_owned(self, merchant_id: uuid.UUID, invoice_id: uuid.UUID) -> InvoiceRow:
        row = await self._repo.get_invoice(invoice_id)
        # Do not leak existence across merchants — 404 for both missing and not-owned.
        if row is None or row.merchant_id != merchant_id:
            raise AppError(
                ErrorCode.INVOICE_NOT_FOUND,
                "invoice not found",
                details={"invoice_id": str(invoice_id)},
            )
        return row

    # ── commands ──
    async def create(
        self,
        *,
        merchant_id: uuid.UUID,
        customer_id: uuid.UUID | None,
        payee_addr: str,
        currency: str,
        lines: list[LineInput],
        due_at: datetime,
    ) -> InvoiceRow:
        totals = compute_totals(lines)
        invoice_id = uuid.uuid4()
        row = InvoiceRow(
            invoice_id=invoice_id,
            merchant_id=merchant_id,
            customer_id=customer_id,
            payee_addr=payee_addr,
            pl_id=None,
            currency=currency,
            subtotal=Decimal(totals.subtotal),
            tax=Decimal(totals.tax),
            total=Decimal(totals.total),
            status=InvoiceStatus.DRAFT.value,
            due_at=due_at,
            paid_at=None,
        )
        await self._repo.insert_invoice(row)
        await self._repo.insert_lines(
            [
                InvoiceLineRow(
                    invoice_id=invoice_id,
                    description=c.description,
                    quantity=c.quantity,
                    unit_price=Decimal(c.unit_price),
                    total=Decimal(c.total),
                    tax_rate=c.tax_rate,
                )
                for c in totals.lines
            ]
        )
        await self._emit(
            invoice_id,
            INVOICE_CREATED,
            {
                "invoice_id": str(invoice_id),
                "merchant_id": str(merchant_id),
                "currency": currency,
                "total": str(totals.total),
                "status": InvoiceStatus.DRAFT.value,
            },
        )
        await self._commit()
        log.info("invoice_created", invoice_id=str(invoice_id), total=totals.total)
        return row

    async def finalize(self, *, merchant_id: uuid.UUID, invoice_id: uuid.UUID) -> InvoiceRow:
        row = await self._load_owned(merchant_id, invoice_id)
        if row.status != InvoiceStatus.DRAFT.value:
            raise AppError(
                ErrorCode.INVALID_STATE,
                f"invoice is {row.status}; only a DRAFT invoice can be finalized",
                details={"status": row.status},
            )
        # Aggregate line totals into a single backing PayLink (the merchant is receiver+creator).
        try:
            pl = await self._paylink.create(
                receiver=row.payee_addr,
                amount=int(row.total),
                currency=row.currency,
                expiry=row.due_at,
                usage="single",
                metadata={"invoice_id": str(invoice_id)},
                # Stable per-invoice key → a retried finalize re-uses the same PayLink (no orphans).
                idempotency_key=f"invoice-{invoice_id}",
            )
        except PaylinkError as exc:
            raise AppError(
                ErrorCode.PAYLINK_UNAVAILABLE,
                "could not create the backing PayLink",
                details={"reason": str(exc)},
            ) from exc
        pl_id = str(pl.get("pl_id") or "")
        if not pl_id:
            raise AppError(ErrorCode.PAYLINK_UNAVAILABLE, "paylink-service returned no pl_id")
        row.pl_id = pl_id
        row.status = InvoiceStatus.OPEN.value
        await self._emit(
            invoice_id,
            INVOICE_FINALIZED,
            {
                "invoice_id": str(invoice_id),
                "pl_id": pl_id,
                "merchant_id": str(merchant_id),
                "total": str(row.total),
                "currency": row.currency,
                "status": InvoiceStatus.OPEN.value,
            },
        )
        await self._commit()
        log.info("invoice_finalized", invoice_id=str(invoice_id), pl_id=pl_id)
        return row

    async def void(self, *, merchant_id: uuid.UUID, invoice_id: uuid.UUID) -> InvoiceRow:
        row = await self._load_owned(merchant_id, invoice_id)
        # With one aggregated usage="single" PayLink, settlement is atomic — the only paid state
        # is PAID, so §2.19's "void blocked after partial pay" reduces to: blocked once PAID.
        if row.status == InvoiceStatus.PAID.value:
            raise AppError(
                ErrorCode.ALREADY_PAID,
                "invoice already paid; cannot void",
                details={"status": row.status},
            )
        if row.status == InvoiceStatus.VOID.value:
            raise AppError(
                ErrorCode.INVALID_STATE, "invoice already void", details={"status": row.status}
            )
        row.status = InvoiceStatus.VOID.value
        await self._emit(
            invoice_id,
            INVOICE_VOIDED,
            {
                "invoice_id": str(invoice_id),
                "merchant_id": str(merchant_id),
                "status": InvoiceStatus.VOID.value,
            },
        )
        await self._commit()
        log.info("invoice_voided", invoice_id=str(invoice_id))
        return row

    # ── queries ──
    async def get(
        self, *, merchant_id: uuid.UUID, invoice_id: uuid.UUID
    ) -> tuple[InvoiceRow, list[InvoiceLineRow]]:
        row = await self._load_owned(merchant_id, invoice_id)
        lines = await self._repo.list_lines(invoice_id)
        return row, lines

    async def list(
        self,
        *,
        merchant_id: uuid.UUID,
        status: str | None = None,
        limit: int = 50,
        offset: int = 0,
    ) -> list[InvoiceRow]:
        return await self._repo.list_invoices(
            merchant_id, status=status, limit=limit, offset=offset
        )

    # ── event-driven transitions (no merchant in the loop) ──
    async def mark_paid_by_plid(self, pl_id: str) -> bool:
        """Settlement truth from chain (``chain.paylink.verified``). Idempotent: only OPEN/OVERDUE
        transition to PAID; a redelivery for an already-PAID invoice is a no-op (no double emit)."""
        row = await self._repo.get_by_plid(pl_id)
        if row is None:
            log.info("paid_event_no_invoice", pl_id=pl_id)
            return False
        if row.status in (InvoiceStatus.OPEN.value, InvoiceStatus.OVERDUE.value):
            row.status = InvoiceStatus.PAID.value
            row.paid_at = datetime.now(UTC)
            await self._emit(
                row.invoice_id,
                INVOICE_PAID,
                {
                    "invoice_id": str(row.invoice_id),
                    "pl_id": pl_id,
                    "merchant_id": str(row.merchant_id),
                    "total": str(row.total),
                    "currency": row.currency,
                    "status": InvoiceStatus.PAID.value,
                },
            )
            await self._commit()
            log.info("invoice_paid", invoice_id=str(row.invoice_id), pl_id=pl_id)
            return True
        log.info("paid_event_skipped", pl_id=pl_id, status=row.status)
        return False

    async def sweep_overdue(self, *, now: datetime | None = None) -> int:
        """Flip OPEN invoices past due_at to OVERDUE and emit ``invoice.overdue`` for each."""
        now = now or datetime.now(UTC)
        rows = await self._repo.find_overdue(now)
        for row in rows:
            row.status = InvoiceStatus.OVERDUE.value
            await self._emit(
                row.invoice_id,
                INVOICE_OVERDUE,
                {
                    "invoice_id": str(row.invoice_id),
                    "merchant_id": str(row.merchant_id),
                    "due_at": row.due_at.isoformat(),
                    "total": str(row.total),
                    "currency": row.currency,
                    "status": InvoiceStatus.OVERDUE.value,
                },
            )
        if rows:
            await self._commit()
            log.info("overdue_swept", count=len(rows))
        return len(rows)
