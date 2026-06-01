"""`/v1/kyc` — start a KYC session (JWT, idempotent) + apply a provider callback (HMAC, no JWT)."""

from __future__ import annotations

import json
from typing import Any

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ProviderRegistryDep, ServicesDep
from app.errors import AppError, ErrorCode
from app.security.authz import require_self_or_admin
from app.security.hmac import verify_signature

router = APIRouter(prefix="/v1/kyc", tags=["kyc"])


@router.post("/sessions", status_code=201)
async def create_session(
    req: schemas.CreateKycSessionRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    registry: ProviderRegistryDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    # Self-or-admin: a user may only start their own KYC (or platform staff may start anyone's).
    require_self_or_admin(principal, req.user_id)
    user_id = parse_uuid(req.user_id, field="user_id")
    provider = registry.default()

    async def work() -> dict[str, Any]:
        started = await services.kyc.create_session(
            provider=provider, user_id=user_id, tier_requested=req.tier_requested
        )
        return schemas.CreateKycSessionResponse(
            session_id=started.session_id, provider_url=started.provider_url
        ).model_dump(mode="json")

    return await idempotent(
        idem,
        "create_session",
        idempotency_key,
        {"user": principal.user_id, **req.model_dump(mode="json")},
        201,
        work,
    )


@router.post("/callbacks/{provider}", status_code=200)
async def callback(
    provider: str,
    request: Request,
    services: ServicesDep,
    registry: ProviderRegistryDep,
) -> JSONResponse:
    # Read the raw body ONCE — the HMAC is computed over these exact bytes, and the same bytes are
    # parsed into the provider callback (re-reading/re-serializing would break the signature check).
    raw = await request.body()

    kyc_provider = registry.get(provider)
    if kyc_provider is None:
        raise AppError(
            ErrorCode.UNKNOWN_PROVIDER, "unknown KYC provider", details={"provider": provider}
        )

    secret = registry.secret_for(provider)
    presented = request.headers.get("X-Signature")
    if not verify_signature(secret, raw, presented):
        raise AppError(ErrorCode.INVALID_SIGNATURE, "invalid or missing X-Signature")

    try:
        body = json.loads(raw.decode() or "{}")
    except (ValueError, UnicodeDecodeError) as exc:
        raise AppError(ErrorCode.INVALID_PAYLOAD, "callback body is not valid JSON") from exc
    if not isinstance(body, dict):
        raise AppError(ErrorCode.INVALID_PAYLOAD, "callback body must be a JSON object")

    try:
        await services.kyc.apply_callback(provider=kyc_provider, body=body)
    except KeyError as exc:
        raise AppError(ErrorCode.INVALID_PAYLOAD, "callback body missing a required field") from exc
    except (ValueError, TypeError) as exc:
        raise AppError(ErrorCode.INVALID_PAYLOAD, "callback body is malformed") from exc

    return JSONResponse(status_code=200, content=schemas.CallbackResponse().model_dump(mode="json"))
