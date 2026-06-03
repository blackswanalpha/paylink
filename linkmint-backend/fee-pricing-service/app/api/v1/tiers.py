"""`GET /v1/pricing/tiers` — list fee tiers. Platform-admin only.

Tier administration is a cross-merchant, platform-wide view, so it is gated on a platform-level role
(``PRICING_ADMIN_USER_ROLES``, default ``admin``), not org membership.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Query

from app.api.v1 import schemas
from app.deps import PrincipalDep, ServicesDep, SettingsDep
from app.domain.rbac import require_platform_admin

router = APIRouter(prefix="/v1/pricing", tags=["pricing"])


@router.get("/tiers", response_model=schemas.TiersResponse)
async def list_tiers(
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
    active: Annotated[bool, Query()] = False,
) -> schemas.TiersResponse:
    require_platform_admin(principal, settings.admin_user_role_set)
    rows = await services.pricing.list_tiers(active_only=active)
    return schemas.TiersResponse(
        tiers=[
            schemas.TierResponse(
                tier=r.tier,
                display_name=r.display_name,
                platform_pct_bps=r.platform_pct_bps,
                platform_fixed=int(r.platform_fixed),
                fixed_currency=r.fixed_currency,
                active=r.active,
            )
            for r in rows
        ]
    )
