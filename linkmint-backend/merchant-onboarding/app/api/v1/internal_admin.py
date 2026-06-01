"""`/internal/admin/merchants` — read-only merchant lookup for the admin-backoffice (work11).

Like ``internal.py`` (the decision endpoint), this lives OUTSIDE the ``/v1`` JWT-authed surface: it
is the internal/consumer entry point the ops console reads through. The gateway/transport authorizes
the caller (admin-backoffice already verified the staff JWT + MFA + scopes), so there is no
per-request principal/RBAC here — unlike ``/v1/merchants/{id}`` which is org-membership-gated and
would hide a merchant from a platform admin. Bank accounts are surfaced by id/status only (never the
encrypted ref or plaintext details), reusing the same redacted response builder.
"""

from __future__ import annotations

from typing import Annotated, Any

from fastapi import APIRouter, Query

from app.api.v1 import schemas
from app.api.v1._helpers import parse_uuid
from app.api.v1.merchants import _merchant_response
from app.deps import ServicesDep
from app.errors import AppError, ErrorCode

router = APIRouter(prefix="/internal/admin/merchants", tags=["internal-admin"])


def _summary(merchant: Any) -> schemas.AdminMerchantSummary:
    return schemas.AdminMerchantSummary(
        merchant_id=str(merchant.merchant_id),
        org_id=str(merchant.org_id),
        business_name=merchant.business_name,
        country=merchant.country,
        type=merchant.type,
        status=merchant.status,
        fee_tier=merchant.fee_tier,
    )


@router.get("/{merchant_id}", response_model=schemas.MerchantResponse)
async def get_merchant(merchant_id: str, services: ServicesDep) -> schemas.MerchantResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    merchant = await services.merchants.get_admin(mid)  # raises MERCHANT_NOT_FOUND (404) if absent
    bank_accounts = await services.merchants.list_bank_accounts_admin(mid)
    return schemas.MerchantResponse.model_validate(_merchant_response(merchant, bank_accounts))


@router.get("", response_model=schemas.AdminMerchantListResponse)
async def search_merchants(
    services: ServicesDep,
    q: Annotated[str, Query(min_length=1, max_length=256)],
    limit: Annotated[int, Query(ge=1, le=100)] = 20,
) -> schemas.AdminMerchantListResponse:
    if not q.strip():
        raise AppError(ErrorCode.INVALID_QUERY, "q must not be blank")
    merchants = await services.merchants.search(q.strip(), limit)
    return schemas.AdminMerchantListResponse(items=[_summary(m) for m in merchants])
