"""FastAPI dependencies. Singletons live on ``app.state`` (wired in :mod:`app.main`); each request
gets a fresh DB session + service bundle.

The auth seam is verifier-only: invoice-subscription is a *consumer* of identity-service's RS256
tokens (not the issuer), so ``get_principal`` verifies the bearer token against identity's public
key and yields the authenticated merchant. Every ``/v1/invoices`` route is owner-scoped to
``principal.user_id`` (the merchant).
"""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Header, Request
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.db.repositories import InvoiceRepository
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
            repo=InvoiceRepository(session),
            commit=session.commit,
            settings=request.app.state.settings,
            publisher=request.app.state.publisher,
            paylink=request.app.state.paylink_client,
        )
        yield build_services(deps)


def get_principal(
    request: Request,
    authorization: str | None = Header(default=None),
) -> AccessClaims:
    """Verify the RS256 bearer token (issued by identity) and return the merchant principal."""
    if not authorization:
        raise AppError(ErrorCode.UNAUTHORIZED, "missing Authorization header")
    parts = authorization.split(" ", 1)
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1].strip():
        raise AppError(ErrorCode.UNAUTHORIZED, "malformed Authorization header")
    verifier: JwtVerifier = request.app.state.jwt_verifier
    return verifier.verify(parts[1].strip())


# Common annotated dependencies.
ServicesDep = Annotated[Services, Depends(get_services)]
SettingsDep = Annotated[Settings, Depends(get_settings)]
IdempotencyDep = Annotated[IdempotencyStore, Depends(get_idempotency)]
PrincipalDep = Annotated[AccessClaims, Depends(get_principal)]
IdemKey = Annotated[str | None, Header(alias="Idempotency-Key")]
