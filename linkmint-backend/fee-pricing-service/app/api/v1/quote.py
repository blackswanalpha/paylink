"""`POST /v1/pricing/quote` — the main pricing API. JWT (any authenticated caller).

Returns one breakdown per (tier, rail). The FX rate (if a conversion applies) is LOCKED at quote
time and stored on each quote row for audit. State-mutating (persists quote rows + emits an event),
so it honours ``Idempotency-Key``.
"""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/pricing", tags=["pricing"])


@router.post("/quote", status_code=200)
async def create_quote(
    req: schemas.QuoteRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    settle_currency = req.settle_currency or req.currency

    async def work() -> dict[str, Any]:
        issued = await services.pricing.quote(
            merchant_id=req.merchant_id,
            gross=req.gross,
            currency=req.currency,
            settle_currency=settle_currency,
            rails=req.rails,
            tiers=req.tiers,
        )
        quotes = [
            schemas.QuoteBreakdown(
                quote_id=str(q.quote_id),
                tier=q.tier,
                rail=q.rail,
                gross=q.gross,
                currency=q.currency,
                gross_settled=q.gross_settled,
                settle_currency=q.settle_currency,
                platform_fee=q.platform_fee,
                rail_fee=q.rail_fee,
                net=q.net,
                fx=q.breakdown["fx"],
                breakdown=q.breakdown,
            )
            for q in issued
        ]
        return schemas.QuoteResponse(quotes=quotes).model_dump(mode="json")

    return await idempotent(
        idem,
        "create_quote",
        idempotency_key,
        {"caller": principal.user_id, **req.model_dump(mode="json")},
        200,
        work,
    )
