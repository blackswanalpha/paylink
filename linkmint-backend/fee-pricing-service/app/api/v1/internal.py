"""Trusted-network surface (``/v1/internal/*``) — NOT exposed at the gateway (catch-all 404s it).

``POST /v1/internal/accruals`` records a realized platform fee (idempotent on ``source_ref``);
``POST /v1/internal/invoices/platform-fee/run`` generates the monthly platform-fee invoices for a
period (idempotent per merchant+period). Both are gated by ``InternalGate`` (X-Internal-Token,
ADR-009) and honour ``Idempotency-Key``.
"""

from __future__ import annotations

from dataclasses import asdict
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent
from app.deps import IdemKey, IdempotencyDep, InternalGateDep, ServicesDep

router = APIRouter(prefix="/v1/internal", tags=["internal"])


@router.post("/accruals", status_code=202)
async def record_accrual(
    req: schemas.AccrualRequest,
    services: ServicesDep,
    _gate: InternalGateDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        result = await services.invoicing.record_accrual(
            merchant_id=req.merchant_id,
            amount=req.amount,
            currency=req.currency,
            source_ref=req.source_ref,
            occurred_at=req.occurred_at,
            quote_id=req.quote_id,
        )
        return schemas.AccrualResponse(accrual_id=result.accrual_id, accepted=True).model_dump(
            mode="json"
        )

    return await idempotent(
        idem, "record_accrual", idempotency_key, req.model_dump(mode="json"), 202, work
    )


@router.post("/invoices/platform-fee/run", status_code=200)
async def run_platform_fee_invoices(
    req: schemas.RunRequest,
    services: ServicesDep,
    _gate: InternalGateDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        result = await services.invoicing.generate_for_period(
            req.period, merchant_id=req.merchant_id
        )
        return schemas.RunResponse(
            period=result.period,
            generated=[schemas.GeneratedInvoiceResponse(**asdict(g)) for g in result.generated],
            skipped_existing=result.skipped_existing,
        ).model_dump(mode="json")

    return await idempotent(
        idem, "run_platform_fee_invoices", idempotency_key, req.model_dump(mode="json"), 200, work
    )
