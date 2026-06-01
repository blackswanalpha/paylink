"""`/v1/paylinks` — create / get / list / cancel."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, Header, Query
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.config import Settings
from app.deps import caller_address, caller_user_id, get_idempotency, get_service, get_settings
from app.domain.service import CreateCommand, PayLinkService
from app.idempotency import IdempotencyStore, fingerprint

router = APIRouter(prefix="/v1/paylinks", tags=["paylinks"])

ServiceDep = Annotated[PayLinkService, Depends(get_service)]
SettingsDep = Annotated[Settings, Depends(get_settings)]
IdemDep = Annotated[IdempotencyStore, Depends(get_idempotency)]
CallerDep = Annotated[str, Depends(caller_address)]
CallerUserDep = Annotated[str | None, Depends(caller_user_id)]
IdemKey = Annotated[str | None, Header(alias="Idempotency-Key")]


@router.post("", status_code=201)
async def create_paylink(
    req: schemas.CreatePayLinkRequest,
    service: ServiceDep,
    settings: SettingsDep,
    idem: IdemDep,
    caller: CallerDep,
    caller_user: CallerUserDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    fp = fingerprint(req.model_dump(mode="json"))
    if idempotency_key:
        cached = await idem.begin("create", idempotency_key, fp)
        if cached is not None:
            return JSONResponse(status_code=cached.http_status, content=cached.body)

    cmd = CreateCommand(
        receiver=req.receiver,
        amount=req.amount,
        currency=req.currency or settings.default_currency,
        expiry=req.expiry,
        usage=req.usage,
        metadata=req.metadata,
        rules=req.rules,
        idem_key=idempotency_key,
        caller_addr=caller,
        user_id=caller_user,
    )
    try:
        row = await service.create(cmd)
    except Exception:
        if idempotency_key:
            await idem.release("create", idempotency_key)
        raise

    body = schemas.CreatePayLinkResponse(
        pl_id=row.pl_id,
        status=row.status,
        created_at=row.created_at,
        chain_tx_hash=row.chain_tx_hash,
    ).model_dump(mode="json")
    if idempotency_key:
        await idem.complete("create", idempotency_key, fp, 201, body)
    return JSONResponse(status_code=201, content=body)


@router.get("/{pl_id}", response_model=schemas.PayLinkResponse)
async def get_paylink(pl_id: str, service: ServiceDep) -> schemas.PayLinkResponse:
    row = await service.get(pl_id)
    return schemas.PayLinkResponse.from_row(row)


@router.get("", response_model=schemas.PayLinkListResponse)
async def list_paylinks(
    service: ServiceDep,
    creator: Annotated[str | None, Query()] = None,
    receiver: Annotated[str | None, Query()] = None,
    status: Annotated[str | None, Query()] = None,
    limit: Annotated[int, Query(ge=1, le=100)] = 20,
    cursor: Annotated[str | None, Query()] = None,
) -> schemas.PayLinkListResponse:
    rows, next_cursor = await service.list(
        creator=creator.lower() if creator else None,
        receiver=receiver.lower() if receiver else None,
        status=status.upper() if status else None,
        limit=limit,
        cursor=cursor,
    )
    return schemas.PayLinkListResponse(
        items=[schemas.PayLinkResponse.from_row(r) for r in rows],
        next_cursor=next_cursor,
    )


@router.post("/{pl_id}/cancel", status_code=200)
async def cancel_paylink(
    pl_id: str,
    service: ServiceDep,
    idem: IdemDep,
    caller: CallerDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    fp = fingerprint({"pl_id": pl_id})
    if idempotency_key:
        cached = await idem.begin("cancel", idempotency_key, fp)
        if cached is not None:
            return JSONResponse(status_code=cached.http_status, content=cached.body)
    try:
        row = await service.cancel(pl_id, caller)
    except Exception:
        if idempotency_key:
            await idem.release("cancel", idempotency_key)
        raise

    body = schemas.CancelResponse(pl_id=row.pl_id, status=row.status).model_dump(mode="json")
    if idempotency_key:
        await idem.complete("cancel", idempotency_key, fp, 200, body)
    return JSONResponse(status_code=200, content=body)
