"""`/v1/users/me` — profile + scoped API keys."""

from __future__ import annotations

import uuid
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.db.models import ApiKeyRow, UserRow
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep
from app.idempotency import fingerprint

router = APIRouter(prefix="/v1/users", tags=["users"])


def _profile(
    user: UserRow, roles: list[tuple[str, str]], user_roles: list[str]
) -> schemas.UserProfileResponse:
    return schemas.UserProfileResponse(
        user_id=str(user.user_id),
        email=user.email,
        phone=user.phone,
        kyc_tier=user.kyc_tier,
        status=user.status,
        roles=[schemas.OrgRoleEntry(org_id=o, role=r) for o, r in roles],
        user_roles=user_roles,
        created_at=user.created_at,
        last_login_at=user.last_login_at,
    )


def _api_key(row: ApiKeyRow) -> schemas.ApiKeyResponse:
    return schemas.ApiKeyResponse(
        api_key_id=str(row.api_key_id),
        org_id=str(row.org_id),
        name=row.name,
        prefix=row.prefix,
        scopes=list(row.scopes),
        status=row.status,
        created_at=row.created_at,
        revoked_at=row.revoked_at,
    )


@router.get("/me", response_model=schemas.UserProfileResponse)
async def get_me(services: ServicesDep, principal: PrincipalDep) -> schemas.UserProfileResponse:
    uid = uuid.UUID(principal.user_id)
    user = await services.users.get(uid)
    roles = await services.users.roles(uid)
    return _profile(user, roles, principal.user_roles)


@router.patch("/me")
async def update_me(
    req: schemas.UpdateProfileRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    uid = uuid.UUID(principal.user_id)

    async def work() -> dict[str, Any]:
        user = await services.users.update(uid, email=req.email, phone=req.phone)
        roles = await services.users.roles(uid)
        return _profile(user, roles, principal.user_roles).model_dump(mode="json")

    return await idempotent(
        idem,
        "update_me",
        idempotency_key,
        {"user": principal.user_id, **req.model_dump(mode="json")},
        200,
        work,
    )


@router.post("/me/api-keys", status_code=201)
async def issue_api_key(
    req: schemas.IssueApiKeyRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    org_id = parse_uuid(req.org_id, field="org_id")
    # Bespoke idempotency: the response carries the one-time secret `full_key`, which we must NOT
    # cache in Redis. On a replay we return the same record with `full_key` redacted to null.
    fp = fingerprint({"user": principal.user_id, **req.model_dump(mode="json")})
    if idempotency_key:
        cached = await idem.begin("issue_api_key", idempotency_key, fp)
        if cached is not None:
            return JSONResponse(status_code=cached.http_status, content=cached.body)
    try:
        row, full_key = await services.api_keys.issue(
            actor_id=uuid.UUID(principal.user_id),
            org_id=org_id,
            name=req.name,
            scopes=[s.value for s in req.scopes],
        )
    except Exception:
        if idempotency_key:
            await idem.release("issue_api_key", idempotency_key)
        raise

    body = schemas.IssueApiKeyResponse(
        api_key_id=str(row.api_key_id),
        org_id=str(row.org_id),
        name=row.name,
        prefix=row.prefix,
        full_key=full_key,
        scopes=list(row.scopes),
        status=row.status,
        created_at=row.created_at,
    ).model_dump(mode="json")
    if idempotency_key:
        await idem.complete("issue_api_key", idempotency_key, fp, 201, {**body, "full_key": None})
    return JSONResponse(status_code=201, content=body)


@router.get("/me/api-keys", response_model=schemas.ApiKeyListResponse)
async def list_api_keys(
    services: ServicesDep, principal: PrincipalDep
) -> schemas.ApiKeyListResponse:
    rows = await services.api_keys.list_for_actor(actor_id=uuid.UUID(principal.user_id))
    return schemas.ApiKeyListResponse(items=[_api_key(r) for r in rows])


@router.delete("/me/api-keys/{api_key_id}")
async def revoke_api_key(
    api_key_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    key_id = parse_uuid(api_key_id, field="api_key_id")

    async def work() -> dict[str, Any]:
        row = await services.api_keys.revoke(
            actor_id=uuid.UUID(principal.user_id), api_key_id=key_id
        )
        return schemas.RevokeApiKeyResponse(
            api_key_id=str(row.api_key_id), status=row.status
        ).model_dump(mode="json")

    return await idempotent(
        idem, "revoke_api_key", idempotency_key, {"api_key_id": api_key_id}, 200, work
    )
