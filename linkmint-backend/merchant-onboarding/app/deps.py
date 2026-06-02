"""FastAPI dependencies. Singletons live on ``app.state`` (wired in :mod:`app.main`); each request
gets a fresh DB session + service bundle.

The auth seam here is verifier-only: merchant-onboarding is a *consumer* of identity-service's
RS256 tokens (not the issuer), so ``get_principal`` verifies the bearer token against identity's
public key and yields the authenticated principal. RBAC is then evaluated from the token claims.
"""

from __future__ import annotations

import ipaddress
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Annotated

from fastapi import Depends, Header, Request
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.db.repositories import MerchantRepository
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
            repo=MerchantRepository(session),
            commit=session.commit,
            settings=request.app.state.settings,
            publisher=request.app.state.publisher,
            bank_cipher=request.app.state.bank_cipher,
            object_store=request.app.state.object_store,
        )
        yield build_services(deps)


def get_principal(
    request: Request,
    authorization: str | None = Header(default=None),
) -> AccessClaims:
    """Verify the RS256 bearer token (issued by identity-service) and return the principal."""
    if not authorization:
        raise AppError(ErrorCode.UNAUTHORIZED, "missing Authorization header")
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1].strip():
        raise AppError(ErrorCode.UNAUTHORIZED, "malformed Authorization header")
    verifier: JwtVerifier = request.app.state.jwt_verifier
    return verifier.verify(parts[1].strip())


@dataclass(frozen=True)
class ClientMeta:
    user_agent: str | None
    ip: str | None


def _valid_ip(host: str | None) -> str | None:
    """Only persist a real IP (the `contracts.ip` column is INET). Non-IP hosts → None."""
    if not host:
        return None
    try:
        ipaddress.ip_address(host)
    except ValueError:
        return None
    return host


def client_meta(request: Request) -> ClientMeta:
    return ClientMeta(
        user_agent=request.headers.get("user-agent"),
        ip=_valid_ip(request.client.host if request.client else None),
    )


# Common annotated dependencies.
ServicesDep = Annotated[Services, Depends(get_services)]
SettingsDep = Annotated[Settings, Depends(get_settings)]
IdempotencyDep = Annotated[IdempotencyStore, Depends(get_idempotency)]
PrincipalDep = Annotated[AccessClaims, Depends(get_principal)]
ClientMetaDep = Annotated[ClientMeta, Depends(client_meta)]
IdemKey = Annotated[str | None, Header(alias="Idempotency-Key")]
