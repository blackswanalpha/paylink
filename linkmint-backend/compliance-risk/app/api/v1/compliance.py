"""`/v1/compliance/status` — read a user's KYC tier, latest risk score, and open flags (JWT)."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Query

from app.api.v1 import schemas
from app.api.v1._helpers import parse_uuid
from app.deps import PrincipalDep, ServicesDep
from app.security.authz import require_self_or_admin

router = APIRouter(prefix="/v1/compliance", tags=["compliance"])


@router.get("/status", response_model=schemas.ComplianceStatusResponse)
async def get_status(
    services: ServicesDep,
    principal: PrincipalDep,
    user_id: Annotated[str, Query(min_length=1)],
) -> schemas.ComplianceStatusResponse:
    # Self-or-admin: a user may read only their own status (or platform staff may read anyone's).
    require_self_or_admin(principal, user_id)
    uid = parse_uuid(user_id, field="user_id")
    view = await services.kyc.get_status(uid)  # raises COMPLIANCE_NOT_FOUND (404) if unknown
    return schemas.ComplianceStatusResponse(
        user_id=view.user_id,
        kyc_tier=view.kyc_tier,
        risk_score=view.risk_score,
        flags=[schemas.ComplianceFlag(**f) for f in view.flags],
    )
