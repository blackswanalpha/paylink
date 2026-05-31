"""`/v1/merchants/{id}/contracts` — list + accept a contract version."""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import ClientMetaDep, IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/merchants", tags=["contracts"])


def _contract_response(contract: Any) -> schemas.ContractResponse:
    return schemas.ContractResponse(
        id=contract.id,
        merchant_id=str(contract.merchant_id),
        version=contract.version,
        accepted_by=str(contract.accepted_by),
        accepted_at=contract.accepted_at,
        ip=str(contract.ip) if contract.ip is not None else None,
        user_agent=contract.user_agent,
    )


@router.get("/{merchant_id}/contracts", response_model=schemas.ContractListResponse)
async def list_contracts(
    merchant_id: str, services: ServicesDep, principal: PrincipalDep
) -> schemas.ContractListResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    contracts = await services.contracts.list_contracts(principal=principal, merchant_id=mid)
    return schemas.ContractListResponse(items=[_contract_response(c) for c in contracts])


@router.post("/{merchant_id}/contracts", status_code=201)
async def accept_contract(
    merchant_id: str,
    req: schemas.AcceptContractRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    client: ClientMetaDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")

    async def work() -> dict[str, Any]:
        contract = await services.contracts.accept_contract(
            principal=principal,
            merchant_id=mid,
            version=req.contract_version,
            accepted=req.accepted,
            ip=client.ip,
            user_agent=client.user_agent,
        )
        return _contract_response(contract).model_dump(mode="json")

    return await idempotent(
        idem,
        "accept_contract",
        idempotency_key,
        {"merchant_id": merchant_id, "user": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )
