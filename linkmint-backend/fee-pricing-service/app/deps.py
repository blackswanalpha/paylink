"""FastAPI dependencies. Singletons live on ``app.state`` (wired in :mod:`app.main`); each request
gets a fresh DB session + service bundle.

Auth is verifier-only: fee-pricing-service is a *consumer* of identity-service's RS256 tokens, so
``get_principal`` verifies the bearer token against identity's public key. The ``/v1/internal/*``
surface uses ``internal_gate`` (trusted network + optional ``X-Internal-Token``) instead of a JWT.
"""

from __future__ import annotations

import hmac
from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Header, Request
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.db.repositories import PricingRepository
from app.domain.services import ServiceDeps, Services, build_services
from app.errors import AppError, ErrorCode
from app.security.jwt import AccessClaims, JwtVerifier


def get_settings(request: Request) -> Settings:
    settings: Settings = request.app.state.settings
    return settings


def get_idempotency(request: Request) -> IdempotencyStore:
    store: IdempotencyStore = request.app.state.idempotency
    return store


async def get_services(request: Request) -> AsyncIterator[Services]:
    sessionmaker = request.app.state.sessionmaker
    async with sessionmaker() as session:
        deps = ServiceDeps(
            repo=PricingRepository(session),
            commit=session.commit,
            settings=request.app.state.settings,
            publisher=request.app.state.publisher,
            fx_provider=request.app.state.fx_provider,
            redis=request.app.state.redis,
            ledger=request.app.state.ledger_poster,
        )
        yield build_services(deps)


def get_principal(
    request: Request,
    authorization: str | None = Header(default=None),
) -> AccessClaims:
    """Verify the RS256 bearer token (issued by identity) and return the principal."""
    if not authorization:
        raise AppError(ErrorCode.UNAUTHORIZED, "missing Authorization header")
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1].strip():
        raise AppError(ErrorCode.UNAUTHORIZED, "malformed Authorization header")
    verifier: JwtVerifier = request.app.state.jwt_verifier
    return verifier.verify(parts[1].strip())


def internal_gate(
    request: Request,
    x_internal_token: str | None = Header(default=None),
) -> None:
    """Guard the internal ``/v1/internal/*`` surface (trusted network + optional shared secret).

    When ``PRICING_INTERNAL_SHARED_SECRET`` is configured, a constant-time ``X-Internal-Token``
    match is required; when unset, the request is allowed (the deployment's network is the control).
    """
    settings: Settings = request.app.state.settings
    secret = settings.internal_shared_secret
    if secret is None:
        return
    expected = secret.get_secret_value()
    if not x_internal_token or not hmac.compare_digest(x_internal_token, expected):
        raise AppError(ErrorCode.UNAUTHORIZED, "invalid or missing X-Internal-Token")


# Common annotated dependencies.
ServicesDep = Annotated[Services, Depends(get_services)]
SettingsDep = Annotated[Settings, Depends(get_settings)]
IdempotencyDep = Annotated[IdempotencyStore, Depends(get_idempotency)]
PrincipalDep = Annotated[AccessClaims, Depends(get_principal)]
InternalGateDep = Annotated[None, Depends(internal_gate)]
IdemKey = Annotated[str | None, Header(alias="Idempotency-Key")]
