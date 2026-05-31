"""FastAPI dependencies. Singletons live on ``app.state`` (wired in :mod:`app.main`); the scope
gate gets a fresh DB session per request.

The auth seam is verifier-only: admin-backoffice is a *consumer* of identity-service's RS256 tokens
(not the issuer), so ``get_principal`` verifies the bearer token and yields the authenticated
principal. Admin-role + MFA + default-deny scope checks then run in :mod:`app.security.authz`.
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Annotated

from fastapi import Depends, Header, Request

from app.audit.sink import AuditSink
from app.config import Settings
from app.db.repositories import AdminRepository
from app.domain.services import Services
from app.errors import AppError, ErrorCode
from app.providers.registry import ProviderRegistry
from app.security.jwt import AccessClaims, JwtVerifier


def get_settings(request: Request) -> Settings:
    settings: Settings = request.app.state.settings
    return settings


def get_providers(request: Request) -> ProviderRegistry:
    providers: ProviderRegistry = request.app.state.providers
    return providers


def get_audit(request: Request) -> AuditSink:
    audit: AuditSink = request.app.state.audit
    return audit


def get_app_services(request: Request) -> Services:
    services: Services = request.app.state.services
    return services


async def get_admin_repo(request: Request) -> AsyncIterator[AdminRepository]:
    sessionmaker = request.app.state.sessionmaker
    async with sessionmaker() as session:
        yield AdminRepository(session)


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


def client_meta(request: Request) -> ClientMeta:
    return ClientMeta(
        user_agent=request.headers.get("user-agent"),
        ip=request.client.host if request.client else None,
    )


# Common annotated dependencies.
SettingsDep = Annotated[Settings, Depends(get_settings)]
ProvidersDep = Annotated[ProviderRegistry, Depends(get_providers)]
AuditDep = Annotated[AuditSink, Depends(get_audit)]
ServicesDep = Annotated[Services, Depends(get_app_services)]
AdminRepoDep = Annotated[AdminRepository, Depends(get_admin_repo)]
PrincipalDep = Annotated[AccessClaims, Depends(get_principal)]
ClientMetaDep = Annotated[ClientMeta, Depends(client_meta)]
