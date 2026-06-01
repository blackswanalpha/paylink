"""`/v1/merchants` — onboard, full-record read, and the admin fee-tier read/change."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/merchants", tags=["merchants"])


def _merchant_response(merchant: Any, bank_accounts: list[Any]) -> dict[str, Any]:
    return schemas.MerchantResponse(
        merchant_id=str(merchant.merchant_id),
        org_id=str(merchant.org_id),
        business_name=merchant.business_name,
        registration_no=merchant.registration_no,
        tax_id=merchant.tax_id,
        country=merchant.country,
        type=merchant.type,
        status=merchant.status,
        fee_tier=merchant.fee_tier,
        onboarded_at=merchant.onboarded_at,
        suspended_at=merchant.suspended_at,
        suspended_reason=merchant.suspended_reason,
        bank_accounts=[
            schemas.BankAccountSummary(
                bank_account_id=str(b.bank_account_id),
                rail=b.rail,
                currency=b.currency,
                status=b.status,
                verified_at=b.verified_at,
            )
            for b in bank_accounts
        ],
    ).model_dump(mode="json")


@router.post("/onboard", status_code=201)
async def onboard(
    req: schemas.OnboardRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    org_id = parse_uuid(req.org_id, field="org_id")

    async def work() -> dict[str, Any]:
        merchant = await services.merchants.onboard(
            principal=principal,
            org_id=org_id,
            business_name=req.business_name,
            registration_no=req.registration_no,
            country=req.country,
            merchant_type=req.type.value,
        )
        return schemas.OnboardResponse(
            merchant_id=str(merchant.merchant_id), status=merchant.status
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "onboard",
        idempotency_key,
        {"user": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.get("/{merchant_id}", response_model=schemas.MerchantResponse)
async def get_merchant(
    merchant_id: str, services: ServicesDep, principal: PrincipalDep
) -> schemas.MerchantResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    merchant = await services.merchants.get_for_member(principal=principal, merchant_id=mid)
    bank_accounts = await services.bank_accounts.list_bank_accounts(
        principal=principal, merchant_id=mid
    )
    return schemas.MerchantResponse.model_validate(_merchant_response(merchant, bank_accounts))


@router.get("/{merchant_id}/fee-tier", response_model=schemas.FeeTierResponse)
async def get_fee_tier(
    merchant_id: str, services: ServicesDep, principal: PrincipalDep
) -> schemas.FeeTierResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    merchant = await services.merchants.get_for_member(principal=principal, merchant_id=mid)
    return schemas.FeeTierResponse(
        merchant_id=str(merchant.merchant_id),
        tier=merchant.fee_tier,
        effective_at=merchant.onboarded_at or datetime.now(UTC),
    )


@router.patch("/{merchant_id}/fee-tier")
async def update_fee_tier(
    merchant_id: str,
    req: schemas.UpdateFeeTierRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")

    async def work() -> dict[str, Any]:
        merchant = await services.merchants.set_fee_tier(
            principal=principal, merchant_id=mid, tier=req.tier.value
        )
        return schemas.FeeTierResponse(
            merchant_id=str(merchant.merchant_id),
            tier=merchant.fee_tier,
            effective_at=merchant.onboarded_at or datetime.now(UTC),
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "update_fee_tier",
        idempotency_key,
        {"merchant_id": merchant_id, **req.model_dump(mode="json")},
        200,
        work,
    )
