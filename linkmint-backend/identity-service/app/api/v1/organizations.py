"""`/v1/organizations` — create org (creator becomes owner) + membership CRUD (RBAC enforced)."""

from __future__ import annotations

import uuid
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/organizations", tags=["organizations"])


@router.post("", status_code=201)
async def create_org(
    req: schemas.CreateOrgRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        org, membership = await services.orgs.create(
            creator_id=uuid.UUID(principal.user_id), name=req.name, org_type=req.type.value
        )
        return schemas.OrgResponse(
            org_id=str(org.org_id),
            name=org.name,
            type=org.type,
            role=membership.role,
            created_at=org.created_at,
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "create_org",
        idempotency_key,
        {"user": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.post("/{org_id}/members", status_code=201)
async def add_member(
    org_id: str,
    req: schemas.AddMemberRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    oid = parse_uuid(org_id, field="org_id")
    target_uid = parse_uuid(req.user_id, field="user_id") if req.user_id else None

    async def work() -> dict[str, Any]:
        membership = await services.orgs.add_member(
            actor_id=uuid.UUID(principal.user_id),
            org_id=oid,
            target_user_id=target_uid,
            target_email=req.email,
            role=req.role.value,
        )
        return schemas.MemberResponse(
            org_id=str(membership.org_id), user_id=str(membership.user_id), role=membership.role
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "add_member",
        idempotency_key,
        {"org_id": org_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.get("/{org_id}/members", response_model=schemas.MemberListResponse)
async def list_members(
    org_id: str, services: ServicesDep, principal: PrincipalDep
) -> schemas.MemberListResponse:
    members = await services.orgs.list_members(
        actor_id=uuid.UUID(principal.user_id), org_id=parse_uuid(org_id, field="org_id")
    )
    return schemas.MemberListResponse(
        items=[
            schemas.MemberResponse(org_id=str(m.org_id), user_id=str(m.user_id), role=m.role)
            for m in members
        ]
    )


@router.delete("/{org_id}/members/{user_id}")
async def remove_member(
    org_id: str,
    user_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    oid = parse_uuid(org_id, field="org_id")
    uid = parse_uuid(user_id, field="user_id")

    async def work() -> dict[str, Any]:
        await services.orgs.remove_member(
            actor_id=uuid.UUID(principal.user_id), org_id=oid, target_user_id=uid
        )
        return {"status": "removed", "org_id": org_id, "user_id": user_id}

    return await idempotent(
        idem, "remove_member", idempotency_key, {"org_id": org_id, "user_id": user_id}, 200, work
    )
