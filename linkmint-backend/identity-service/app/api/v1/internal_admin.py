"""`/internal/admin/users` — read-only user lookup for the admin-backoffice (work11).

This lives OUTSIDE the ``/v1`` JWT-authed surface (like the health/metrics probes): it is the
internal/consumer entry point the ops console reads through. In the deployed topology the
gateway/transport authorizes the caller (admin-backoffice verifies the staff JWT + MFA + scopes
before it ever reaches here); there is no per-request principal/RBAC by design — and it never
returns secrets (no password/MFA/refresh hashes).
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Query

from app.api.v1 import schemas
from app.api.v1._helpers import parse_uuid
from app.db.models import UserRow
from app.deps import ServicesDep
from app.errors import AppError, ErrorCode

router = APIRouter(prefix="/internal/admin/users", tags=["internal-admin"])


def _view(user: UserRow) -> schemas.AdminUserView:
    return schemas.AdminUserView(
        user_id=str(user.user_id),
        email=user.email,
        phone=user.phone,
        kyc_tier=user.kyc_tier,
        status=user.status,
        created_at=user.created_at,
        last_login_at=user.last_login_at,
    )


@router.get("/{user_id}", response_model=schemas.AdminUserView)
async def get_user(user_id: str, services: ServicesDep) -> schemas.AdminUserView:
    uid = parse_uuid(user_id, field="user_id")
    user = await services.users.get(uid)  # raises USER_NOT_FOUND (404) when absent
    return _view(user)


@router.get("", response_model=schemas.AdminUserListResponse)
async def search_users(
    services: ServicesDep,
    q: Annotated[str, Query(min_length=1, max_length=256)],
    limit: Annotated[int, Query(ge=1, le=100)] = 20,
) -> schemas.AdminUserListResponse:
    if not q.strip():
        raise AppError(ErrorCode.INVALID_QUERY, "q must not be blank")
    users = await services.users.search(q.strip(), limit)
    return schemas.AdminUserListResponse(items=[_view(u) for u in users])
