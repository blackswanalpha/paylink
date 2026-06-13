"""`/v1/disputes` — rail webhook intake (HMAC, no JWT) + evidence/submit/read (JWT).

The provider webhook is NOT JWT-authed: rail providers can't present a LinkMint token, so the trust
anchor is a per-provider HMAC secret over the raw body (mirrors compliance-risk's KYC callbacks).
Intake AND resolution arrive on the same route, dispatched on the payload ``kind``. The other routes
are org-scoped JWT (evidence: any member; submit: org admin).
"""

from __future__ import annotations

import json
from typing import Any

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.config import Settings
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep, SettingsDep
from app.domain.rbac import require_org_admin, require_org_member
from app.errors import AppError, ErrorCode
from app.security.hmac import verify_signature

router = APIRouter(prefix="/v1/disputes", tags=["disputes"])


@router.post("/webhooks/{provider}", status_code=200)
async def dispute_webhook(
    provider: str,
    request: Request,
    services: ServicesDep,
) -> JSONResponse:
    # Read the raw body ONCE — the HMAC is computed over these exact bytes, and the same bytes are
    # parsed into the webhook payload (re-reading/re-serializing would break the signature check).
    raw = await request.body()

    settings: Settings = request.app.state.settings
    secrets = settings.webhook_secrets_map
    if provider not in secrets:
        raise AppError(
            ErrorCode.UNKNOWN_PROVIDER, "unknown dispute provider", details={"provider": provider}
        )

    presented = request.headers.get("X-Signature")
    if not verify_signature(secrets[provider], raw, presented):
        raise AppError(ErrorCode.INVALID_SIGNATURE, "invalid or missing X-Signature")

    try:
        body = json.loads(raw.decode() or "{}")
    except (ValueError, UnicodeDecodeError) as exc:
        raise AppError(ErrorCode.INVALID_PAYLOAD, "webhook body is not valid JSON") from exc
    if not isinstance(body, dict):
        raise AppError(ErrorCode.INVALID_PAYLOAD, "webhook body must be a JSON object")

    try:
        result = await services.disputes.intake(provider=provider, body=body)
    except (KeyError, TypeError, ValueError) as exc:
        raise AppError(ErrorCode.INVALID_PAYLOAD, "webhook body is malformed") from exc

    return JSONResponse(
        status_code=200,
        content=schemas.WebhookResponse(
            action=result.action, dispute_id=result.dispute_id
        ).model_dump(mode="json"),
    )


@router.post("/{dispute_id}/evidence", status_code=201)
async def add_evidence(
    dispute_id: str,
    req: schemas.AddEvidenceRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    did = parse_uuid(dispute_id, field="dispute_id")
    existing, _ = await services.disputes.get(did)
    require_org_member(
        principal,
        str(existing.org_id) if existing.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )

    async def work() -> dict[str, Any]:
        row = await services.disputes.add_evidence(
            dispute_id=did,
            kind=req.kind,
            summary=req.summary,
            payload=req.payload,
            external_ref=req.external_ref,
            submitted_by=principal.user_id,
        )
        return schemas.EvidenceView.from_row(row).model_dump(mode="json")

    return await idempotent(
        idem,
        "add_evidence",
        idempotency_key,
        {"caller": principal.user_id, "id": dispute_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.post("/{dispute_id}/submit", status_code=200)
async def submit_dispute(
    dispute_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    did = parse_uuid(dispute_id, field="dispute_id")
    existing, _ = await services.disputes.get(did)
    require_org_admin(
        principal,
        str(existing.org_id) if existing.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )

    async def work() -> dict[str, Any]:
        row = await services.disputes.submit(did, submitted_by=principal.user_id)
        evidence = await services.disputes.get(did)
        return schemas.DisputeView.from_row(row, evidence[1]).model_dump(mode="json")

    return await idempotent(
        idem,
        "submit_dispute",
        idempotency_key,
        {"caller": principal.user_id, "id": dispute_id},
        200,
        work,
    )


@router.get("/{dispute_id}", status_code=200)
async def get_dispute(
    dispute_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    settings: SettingsDep,
) -> schemas.DisputeView:
    row, evidence = await services.disputes.get(parse_uuid(dispute_id, field="dispute_id"))
    require_org_member(
        principal,
        str(row.org_id) if row.org_id else None,
        platform_roles=settings.admin_user_role_set,
    )
    return schemas.DisputeView.from_row(row, evidence)
