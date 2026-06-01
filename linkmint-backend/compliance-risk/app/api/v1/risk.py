"""`/v1/risk/evaluate` — the INTERNAL risk-decision endpoint (no JWT; trusted net + InternalGate).

Consumed by paylink-service for above-threshold PayLink creation (Flow E). The request/response
shapes are a FIXED contract — see :mod:`app.api.v1.schemas`. The ``InternalGate`` dependency allows
the call on the trusted network, optionally hardened by a constant-time ``X-Internal-Token`` match.
"""

from __future__ import annotations

from fastapi import APIRouter

from app.api.v1 import schemas
from app.api.v1._helpers import parse_uuid
from app.deps import InternalGateDep, ServicesDep

router = APIRouter(prefix="/v1/risk", tags=["risk"])


@router.post("/evaluate", response_model=schemas.RiskEvaluateResponse)
async def evaluate(
    req: schemas.RiskEvaluateRequest,
    services: ServicesDep,
    _gate: InternalGateDep,
) -> schemas.RiskEvaluateResponse:
    user_id = parse_uuid(req.user_id, field="user_id")
    outcome = await services.risk.evaluate(
        user_id=user_id,
        action=req.action,
        amount=req.amount,
        currency=req.currency,
        geo_country=req.geo,
        registered_country=req.registered_country,
        context=req.context,
    )
    return schemas.RiskEvaluateResponse(
        decision=outcome.decision.value,
        score=outcome.score,
        reasons=[schemas.RiskReason(code=r.code, detail=r.detail) for r in outcome.reasons],
    )
