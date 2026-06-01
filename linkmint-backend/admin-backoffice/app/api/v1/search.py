"""`GET /v1/admin/search` — unified search across users, merchants, PayLinks, payments."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, Query

from app.api.v1 import schemas
from app.deps import ClientMetaDep, ServicesDep
from app.security.authz import AdminContext, require_admin

router = APIRouter(prefix="/v1/admin", tags=["admin"])


@router.get("/search", response_model=schemas.SearchResponse)
async def search(
    services: ServicesDep,
    client: ClientMetaDep,
    q: Annotated[str, Query(min_length=1, max_length=256)],
    admin: Annotated[AdminContext, Depends(require_admin("support.read"))],
) -> schemas.SearchResponse:
    result = await services.search.search(
        principal=admin.principal,
        scopes=admin.scopes,
        ip=client.ip,
        user_agent=client.user_agent,
        q=q,
    )
    return schemas.SearchResponse.from_result(result)
