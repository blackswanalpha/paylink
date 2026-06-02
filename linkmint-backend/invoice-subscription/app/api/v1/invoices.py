"""`/v1/invoices` — create (DRAFT) + finalize (→ PayLink, OPEN) + void + read. JWT (merchant).

Every route is owner-scoped to the authenticated merchant (``principal.user_id``). State-mutating
routes honour ``Idempotency-Key``. Reads reflect OVERDUE lazily (an OPEN invoice past ``due_at``
reads as OVERDUE even before the sweeper persists the transition).
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Annotated, Any

from fastapi import APIRouter, Query
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.db.models import InvoiceLineRow, InvoiceRow
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep, SettingsDep
from app.domain.models import InvoiceStatus, LineInput, effective_status
from app.errors import AppError, ErrorCode

router = APIRouter(prefix="/v1/invoices", tags=["invoices"])


def _summary(row: InvoiceRow, now: datetime) -> schemas.InvoiceSummaryResponse:
    return schemas.InvoiceSummaryResponse(
        invoice_id=str(row.invoice_id),
        merchant_id=str(row.merchant_id),
        customer_id=str(row.customer_id) if row.customer_id else None,
        payee_addr=row.payee_addr,
        pl_id=row.pl_id,
        currency=row.currency,
        subtotal=int(row.subtotal),
        tax=int(row.tax),
        total=int(row.total),
        status=effective_status(row.status, row.due_at, now),
        due_at=row.due_at,
        paid_at=row.paid_at,
        created_at=row.created_at,
        updated_at=row.updated_at,
    )


def _line(row: InvoiceLineRow) -> schemas.InvoiceLineResponse:
    return schemas.InvoiceLineResponse(
        description=row.description,
        quantity=str(row.quantity),
        unit_price=int(row.unit_price),
        total=int(row.total),
        tax_rate=str(row.tax_rate),
    )


def _detail(row: InvoiceRow, lines: list[InvoiceLineRow], now: datetime) -> schemas.InvoiceResponse:
    summary = _summary(row, now)
    return schemas.InvoiceResponse(**summary.model_dump(), lines=[_line(ln) for ln in lines])


@router.post("", status_code=201)
async def create_invoice(
    req: schemas.CreateInvoiceRequest,
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    merchant_id = parse_uuid(principal.user_id, field="merchant_id")
    currency = (req.currency or settings.default_currency).upper()
    lines = [
        LineInput(
            description=ln.description,
            quantity=ln.quantity,
            unit_price=ln.unit_price,
            tax_rate=ln.tax_rate,
        )
        for ln in req.lines
    ]

    async def work() -> dict[str, Any]:
        row = await services.invoices.create(
            merchant_id=merchant_id,
            customer_id=req.customer_id,
            payee_addr=req.payee_addr,
            currency=currency,
            lines=lines,
            due_at=req.due_at,
        )
        return schemas.CreateInvoiceResponse(
            invoice_id=str(row.invoice_id), pl_id=row.pl_id, status=row.status
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "create_invoice",
        idempotency_key,
        {"merchant": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.get("", response_model=schemas.InvoiceListResponse)
async def list_invoices(
    services: ServicesDep,
    principal: PrincipalDep,
    status: Annotated[str | None, Query()] = None,
    limit: Annotated[int, Query(ge=1, le=200)] = 50,
    offset: Annotated[int, Query(ge=0)] = 0,
) -> schemas.InvoiceListResponse:
    merchant_id = parse_uuid(principal.user_id, field="merchant_id")
    if status is not None and status not in set(InvoiceStatus):
        raise AppError(ErrorCode.INVALID_QUERY, "invalid status filter", details={"status": status})
    rows = await services.invoices.list(
        merchant_id=merchant_id, status=status, limit=limit, offset=offset
    )
    now = datetime.now(UTC)
    return schemas.InvoiceListResponse(
        items=[_summary(r, now) for r in rows], limit=limit, offset=offset, count=len(rows)
    )


@router.get("/{invoice_id}", response_model=schemas.InvoiceResponse)
async def get_invoice(
    invoice_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
) -> schemas.InvoiceResponse:
    merchant_id = parse_uuid(principal.user_id, field="merchant_id")
    iid = parse_uuid(invoice_id, field="invoice_id")
    row, lines = await services.invoices.get(merchant_id=merchant_id, invoice_id=iid)
    return _detail(row, lines, datetime.now(UTC))


@router.post("/{invoice_id}/finalize", status_code=200)
async def finalize_invoice(
    invoice_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    merchant_id = parse_uuid(principal.user_id, field="merchant_id")
    iid = parse_uuid(invoice_id, field="invoice_id")

    async def work() -> dict[str, Any]:
        row = await services.invoices.finalize(merchant_id=merchant_id, invoice_id=iid)
        return schemas.FinalizeResponse(
            invoice_id=str(row.invoice_id), status=row.status, pl_id=row.pl_id or ""
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "finalize_invoice",
        idempotency_key,
        {"merchant": principal.user_id, "invoice_id": invoice_id},
        200,
        work,
    )


@router.post("/{invoice_id}/void", status_code=200)
async def void_invoice(
    invoice_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    merchant_id = parse_uuid(principal.user_id, field="merchant_id")
    iid = parse_uuid(invoice_id, field="invoice_id")

    async def work() -> dict[str, Any]:
        row = await services.invoices.void(merchant_id=merchant_id, invoice_id=iid)
        return schemas.VoidResponse(invoice_id=str(row.invoice_id), status=row.status).model_dump(
            mode="json"
        )

    return await idempotent(
        idem,
        "void_invoice",
        idempotency_key,
        {"merchant": principal.user_id, "invoice_id": invoice_id},
        200,
        work,
    )
