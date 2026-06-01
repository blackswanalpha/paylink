"""`/internal/merchants/{id}/decision` — the manual-review + work11/work15 state-machine entry.

This lives OUTSIDE the ``/v1`` JWT-authed surface (like the health/metrics probes): it is the
internal/consumer entry point that drives ``MerchantsService.decide`` (approve|reject|suspend|
reinstate). In the deployed topology the gateway/transport authorizes the caller; admin-backoffice
(work11) and the event bus (work15) call this same guarded path. No principal/RBAC here by design.
"""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, ServicesDep

router = APIRouter(prefix="/internal/merchants", tags=["internal"])


@router.post("/{merchant_id}/decision", status_code=200)
async def decide(
    merchant_id: str,
    req: schemas.DecisionRequest,
    services: ServicesDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")

    async def work() -> dict[str, Any]:
        merchant = await services.merchants.decide(
            merchant_id=mid, decision=req.decision, reason=req.reason
        )
        return schemas.DecisionResponse(
            merchant_id=str(merchant.merchant_id), status=merchant.status
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "decision",
        idempotency_key,
        {"merchant_id": merchant_id, **req.model_dump(mode="json")},
        200,
        work,
    )
