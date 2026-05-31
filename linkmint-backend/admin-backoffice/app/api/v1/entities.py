"""`GET /v1/admin/{users,merchants,paylinks,payments}/{id}` — entity drill-down read views."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends

from app.api.v1 import schemas
from app.deps import ClientMetaDep, ServicesDep
from app.security.authz import AdminContext, require_admin

router = APIRouter(prefix="/v1/admin", tags=["admin"])

# All Phase-1 read views require the support.read scope (one gate object, reused).
_READ = require_admin("support.read")


async def _view(
    services: ServicesDep,
    admin: AdminContext,
    client: ClientMetaDep,
    *,
    entity_type: str,
    entity_id: str,
) -> schemas.EntityResponse:
    view = await services.entity.get(
        principal=admin.principal,
        scopes=admin.scopes,
        ip=client.ip,
        user_agent=client.user_agent,
        entity_type=entity_type,
        entity_id=entity_id,
    )
    return schemas.EntityResponse.from_view(view)


@router.get("/users/{entity_id}", response_model=schemas.EntityResponse)
async def get_user(
    entity_id: str,
    services: ServicesDep,
    client: ClientMetaDep,
    admin: Annotated[AdminContext, Depends(_READ)],
) -> schemas.EntityResponse:
    return await _view(services, admin, client, entity_type="user", entity_id=entity_id)


@router.get("/merchants/{entity_id}", response_model=schemas.EntityResponse)
async def get_merchant(
    entity_id: str,
    services: ServicesDep,
    client: ClientMetaDep,
    admin: Annotated[AdminContext, Depends(_READ)],
) -> schemas.EntityResponse:
    return await _view(services, admin, client, entity_type="merchant", entity_id=entity_id)


@router.get("/paylinks/{entity_id}", response_model=schemas.EntityResponse)
async def get_paylink(
    entity_id: str,
    services: ServicesDep,
    client: ClientMetaDep,
    admin: Annotated[AdminContext, Depends(_READ)],
) -> schemas.EntityResponse:
    return await _view(services, admin, client, entity_type="paylink", entity_id=entity_id)


@router.get("/payments/{entity_id}", response_model=schemas.EntityResponse)
async def get_payment(
    entity_id: str,
    services: ServicesDep,
    client: ClientMetaDep,
    admin: Annotated[AdminContext, Depends(_READ)],
) -> schemas.EntityResponse:
    return await _view(services, admin, client, entity_type="payment", entity_id=entity_id)
