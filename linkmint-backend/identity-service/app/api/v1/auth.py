"""`/v1/auth/*` — register, login, refresh, logout, OAuth, MFA, and the JWKS/OIDC metadata."""

from __future__ import annotations

import uuid
from typing import Any

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent
from app.deps import ClientMetaDep, IdemKey, IdempotencyDep, PrincipalDep, ServicesDep, SettingsDep
from app.domain.models import AuthTokens
from app.security.keys import KeyStore

router = APIRouter(prefix="/v1/auth", tags=["auth"])


def _tokens(t: AuthTokens) -> schemas.TokenResponse:
    return schemas.TokenResponse(
        access_token=t.access_token,
        refresh_token=t.refresh_token,
        token_type=t.token_type,
        expires_in=t.expires_in,
    )


@router.get("/.well-known/jwks.json")
async def jwks(request: Request) -> dict[str, Any]:
    keys: KeyStore = request.app.state.keys
    return keys.jwks()


@router.get("/.well-known/openid-configuration")
async def openid_configuration(request: Request, settings: SettingsDep) -> dict[str, Any]:
    base = str(request.base_url).rstrip("/")
    return {
        "issuer": settings.jwt_issuer,
        "jwks_uri": f"{base}/v1/auth/.well-known/jwks.json",
        "id_token_signing_alg_values_supported": ["RS256"],
        "response_types_supported": ["token"],
        "subject_types_supported": ["public"],
    }


@router.post("/register", status_code=201)
async def register(
    req: schemas.RegisterRequest,
    services: ServicesDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        user = await services.auth.register(email=req.email, phone=req.phone, password=req.password)
        return schemas.RegisterResponse(user_id=str(user.user_id), status=user.status).model_dump(
            mode="json"
        )

    return await idempotent(
        idem, "register", idempotency_key, req.model_dump(mode="json"), 201, work
    )


@router.post("/login")
async def login(
    req: schemas.LoginRequest, services: ServicesDep, meta: ClientMetaDep
) -> schemas.TokenResponse:
    tokens = await services.auth.login(
        identifier=req.identifier,
        password=req.password,
        mfa_code=req.mfa_code,
        user_agent=meta.user_agent,
        ip=meta.ip,
    )
    return _tokens(tokens)


@router.post("/refresh")
async def refresh(
    req: schemas.RefreshRequest, services: ServicesDep, meta: ClientMetaDep
) -> schemas.TokenResponse:
    tokens = await services.auth.refresh(req.refresh_token, user_agent=meta.user_agent, ip=meta.ip)
    return _tokens(tokens)


@router.post("/logout")
async def logout(
    req: schemas.LogoutRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        await services.auth.logout(uuid.UUID(principal.user_id), req.refresh_token)
        return {"status": "logged_out"}

    return await idempotent(
        idem, "logout", idempotency_key, {"refresh_token": req.refresh_token}, 200, work
    )


@router.post("/oauth/{provider}/start")
async def oauth_start(
    provider: str, req: schemas.OAuthStartRequest, services: ServicesDep
) -> schemas.OAuthStartResponse:
    authz = services.auth.oauth_start(provider, state=req.state, redirect_uri=req.redirect_uri)
    return schemas.OAuthStartResponse(authorize_url=authz.authorize_url, state=authz.state)


@router.post("/oauth/{provider}/callback")
async def oauth_callback(
    provider: str,
    req: schemas.OAuthCallbackRequest,
    services: ServicesDep,
    meta: ClientMetaDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        tokens = await services.auth.oauth_callback(
            provider,
            code=req.code,
            state=req.state,
            redirect_uri=req.redirect_uri,
            user_agent=meta.user_agent,
            ip=meta.ip,
        )
        return _tokens(tokens).model_dump(mode="json")

    return await idempotent(
        idem,
        "oauth_callback",
        idempotency_key,
        {"provider": provider, **req.model_dump(mode="json")},
        200,
        work,
    )


@router.post("/mfa/enroll")
async def mfa_enroll(services: ServicesDep, principal: PrincipalDep) -> schemas.MfaEnrollResponse:
    # Not idempotency-keyed on purpose: the response carries a fresh TOTP secret we must not cache.
    user = await services.users.get(uuid.UUID(principal.user_id))
    account = user.email or user.phone or str(user.user_id)
    secret, uri = await services.mfa.enroll(uuid.UUID(principal.user_id), account_name=account)
    return schemas.MfaEnrollResponse(secret=secret, otpauth_uri=uri)


@router.post("/mfa/verify")
async def mfa_verify(
    req: schemas.MfaCodeRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        await services.mfa.verify(uuid.UUID(principal.user_id), req.code)
        return schemas.MfaVerifyResponse(enabled=True).model_dump(mode="json")

    return await idempotent(
        idem,
        "mfa_verify",
        idempotency_key,
        {"user": principal.user_id, "code": req.code},
        200,
        work,
    )


@router.post("/mfa/disable")
async def mfa_disable(
    req: schemas.MfaCodeRequest,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    async def work() -> dict[str, Any]:
        await services.mfa.disable(uuid.UUID(principal.user_id), req.code)
        return {"status": "disabled"}

    return await idempotent(
        idem,
        "mfa_disable",
        idempotency_key,
        {"user": principal.user_id, "code": req.code},
        200,
        work,
    )
