"""A merchant's current pricing config. JWT (org-member of that merchant, or platform admin).

Served at BOTH the spec path ``/v1/merchants/{id}/pricing`` (east-west callers) and the
gateway-facing ``/v1/pricing/merchants/{id}`` (avoids the Kong ``/v1/merchants`` prefix collision
with merchant-onboarding). No-leak: a non-member (or a merchant whose org mapping isn't known yet)
gets the SAME 404 as a missing merchant — existence is never revealed.
"""

from __future__ import annotations

from fastapi import APIRouter

from app.api.v1 import schemas
from app.api.v1._helpers import parse_uuid
from app.deps import PrincipalDep, ServicesDep, SettingsDep
from app.domain.rbac import is_platform_admin, org_role
from app.errors import AppError, ErrorCode

router = APIRouter(tags=["pricing"])


async def _read(
    merchant_id: str,
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
) -> schemas.MerchantPricingResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    view = await services.pricing.get_merchant_pricing_view(mid)

    # Authorize: platform admin sees any merchant; otherwise the caller must be a member of the
    # merchant's org. A non-member (or unknown org) gets the same NOT_FOUND — no existence leak.
    if not is_platform_admin(principal, settings.admin_user_role_set):
        is_member = view.org_id is not None and org_role(principal, view.org_id) is not None
        if not is_member:
            raise AppError(
                ErrorCode.MERCHANT_PRICING_NOT_FOUND,
                "no pricing configured for merchant",
                details={"merchant_id": merchant_id},
            )

    return schemas.MerchantPricingResponse(
        merchant_id=view.merchant_id,
        org_id=view.org_id,
        tier=view.tier,
        display_name=view.display_name,
        platform_pct_bps=view.platform_pct_bps,
        platform_fixed=view.platform_fixed,
        fixed_currency=view.fixed_currency,
        rail_fees=[
            schemas.MerchantRailFeeResponse(
                rail=rf.rail, pct_bps=rf.pct_bps, fixed=rf.fixed, fixed_currency=rf.fixed_currency
            )
            for rf in view.rail_fees
        ],
        effective_at=view.effective_at,
    )


@router.get("/v1/pricing/merchants/{merchant_id}", response_model=schemas.MerchantPricingResponse)
async def get_merchant_pricing_namespaced(
    merchant_id: str,
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
) -> schemas.MerchantPricingResponse:
    return await _read(merchant_id, services, settings, principal)


@router.get("/v1/merchants/{merchant_id}/pricing", response_model=schemas.MerchantPricingResponse)
async def get_merchant_pricing_spec(
    merchant_id: str,
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
) -> schemas.MerchantPricingResponse:
    return await _read(merchant_id, services, settings, principal)
