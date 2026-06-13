"""`/v1/refunds` — request a refund, approve/reject it, and read it back. JWT (RS256 self-verify).

A refund is org-scoped: when the request carries an ``org_id`` the caller must be a member (approve/
reject require an org admin). Sender-initiated refunds (no org_id) may be requested by any
authenticated user, but approval — the gate that triggers the rail reversal — then requires a
platform admin. Reads/approvals are authorized against the stored refund's ``org_id``.
"""

from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Query
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep, SettingsDep
from app.domain.rbac import require_org_admin, require_org_member

router = APIRouter(prefix="/v1/refunds", tags=["refunds"])


@router.post("", status_code=201)
async def create_refund(
    req: schemas.CreateRefundRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    org_id = parse_uuid(req.org_id, field="org_id") if req.org_id else None
    merchant_id = parse_uuid(req.merchant_id, field="merchant_id") if req.merchant_id else None
    # If the refund is org-scoped, the caller must belong to that org (membership is the entry gate;
    # approval is the stronger gate). An unscoped (sender-initiated) refund is allowed for any
    # authenticated principal.
    if org_id is not None:
        require_org_member(principal, str(org_id), platform_roles=settings.admin_user_role_set)

    async def work() -> dict[str, Any]:
        row = await services.refunds.request_refund(
            payment_id=req.payment_id,
            amount_minor=req.amount_minor,
            currency=req.currency,
            reason=req.reason,
            requested_by=principal.user_id,
            org_id=org_id,
            merchant_id=merchant_id,
        )
        return schemas.RefundView.from_row(row).model_dump(mode="json")

    return await idempotent(
        idem,
        "create_refund",
        idempotency_key,
        {"caller": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.post("/{refund_id}/approve", status_code=200)
async def approve_refund(
    refund_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    rid = parse_uuid(refund_id, field="refund_id")
    existing = await services.refunds.get(rid)
    require_org_admin(
        principal,
        str(existing.org_id) if existing.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )

    async def work() -> dict[str, Any]:
        row = await services.refunds.approve(rid, approved_by=principal.user_id)
        return schemas.RefundView.from_row(row).model_dump(mode="json")

    return await idempotent(
        idem,
        "approve_refund",
        idempotency_key,
        {"caller": principal.user_id, "id": refund_id},
        200,
        work,
    )


@router.post("/{refund_id}/reject", status_code=200)
async def reject_refund(
    refund_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    rid = parse_uuid(refund_id, field="refund_id")
    existing = await services.refunds.get(rid)
    require_org_admin(
        principal,
        str(existing.org_id) if existing.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )

    async def work() -> dict[str, Any]:
        row = await services.refunds.reject(rid, rejected_by=principal.user_id)
        return schemas.RefundView.from_row(row).model_dump(mode="json")

    return await idempotent(
        idem,
        "reject_refund",
        idempotency_key,
        {"caller": principal.user_id, "id": refund_id},
        200,
        work,
    )


@router.get("/{refund_id}", status_code=200)
async def get_refund(
    refund_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
) -> schemas.RefundView:
    row = await services.refunds.get(parse_uuid(refund_id, field="refund_id"))
    require_org_member(
        principal,
        str(row.org_id) if row.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )
    return schemas.RefundView.from_row(row)


@router.get("", status_code=200)
async def list_refunds(
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    payment_id: str = Query(..., min_length=1),
) -> schemas.RefundListResponse:
    rows = await services.refunds.list_by_payment(payment_id)
    # Filter to refunds the caller may see (org member or platform admin); unscoped rows are
    # admin-only. This never leaks the existence of other orgs' refunds.
    allowed = []
    is_admin = bool(settings.admin_user_role_set & set(principal.user_roles))
    member_orgs = {r.org_id for r in principal.roles}
    for row in rows:
        oid = str(row.org_id) if row.org_id else None
        if is_admin or (oid is not None and oid in member_orgs):
            allowed.append(row)
    return schemas.RefundListResponse(refunds=[schemas.RefundView.from_row(r) for r in allowed])
