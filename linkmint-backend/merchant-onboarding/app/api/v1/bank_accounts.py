"""`/v1/merchants/{id}/bank-accounts` — link a bank account (+ verify).

SECURITY: the raw ``account_details`` are NEVER placed in the idempotency fingerprint payload nor in
the cached/returned body — only the account id + status. The plaintext is encrypted to
``account_ref`` inside the service and discarded.
"""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/merchants", tags=["bank-accounts"])


@router.post("/{merchant_id}/bank-accounts", status_code=201)
async def add_bank_account(
    merchant_id: str,
    req: schemas.AddBankAccountRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    # NB: account_details is deliberately EXCLUDED from the fingerprint payload — the secret never
    # enters Redis. rail+currency+country key the idempotent identity of the add.
    fp_payload = {
        "merchant_id": merchant_id,
        "rail": req.rail.value,
        "currency": req.currency,
        "country": req.country,
    }

    async def work() -> dict[str, Any]:
        account = await services.bank_accounts.add_bank_account(
            principal=principal,
            merchant_id=mid,
            rail=req.rail.value,
            account_details=req.account_details,
            currency=req.currency,
            country=req.country,
        )
        return schemas.AddBankAccountResponse(
            bank_account_id=str(account.bank_account_id), status=account.status
        ).model_dump(mode="json")

    return await idempotent(idem, "add_bank_account", idempotency_key, fp_payload, 201, work)


@router.post("/{merchant_id}/bank-accounts/{bank_account_id}/verify", status_code=200)
async def verify_bank_account(
    merchant_id: str,
    bank_account_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    req: schemas.VerifyBankAccountRequest | None = None,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    bid = parse_uuid(bank_account_id, field="bank_account_id")

    async def work() -> dict[str, Any]:
        account = await services.bank_accounts.verify_bank_account(
            principal=principal, merchant_id=mid, bank_account_id=bid
        )
        return schemas.VerifyBankAccountResponse(
            bank_account_id=str(account.bank_account_id), status=account.status
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "verify_bank_account",
        idempotency_key,
        {"merchant_id": merchant_id, "bank_account_id": bank_account_id},
        200,
        work,
    )
