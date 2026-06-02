"""FastAPI dependencies. Singletons live on ``app.state`` (wired in :mod:`app.main`); each request
gets a fresh DB session + service. Tests override these to inject fakes without Docker."""

from __future__ import annotations

from collections.abc import AsyncIterator

from fastapi import Depends, Header, Request
from linkmint_idempotency import IdempotencyStore
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.config import Settings
from app.db.repository import PayLinkRepository
from app.domain.service import PayLinkService


def get_settings(request: Request) -> Settings:
    settings: Settings = request.app.state.settings
    return settings


def get_idempotency(request: Request) -> IdempotencyStore:
    store: IdempotencyStore = request.app.state.idempotency
    return store


async def get_service(request: Request) -> AsyncIterator[PayLinkService]:
    sessionmaker: async_sessionmaker[AsyncSession] = request.app.state.sessionmaker
    async with sessionmaker() as session:
        yield PayLinkService(
            repo=PayLinkRepository(session),
            commit=session.commit,
            chain=request.app.state.chain_client,
            signer=request.app.state.signer,
            nonces=request.app.state.nonces,
            publisher=request.app.state.publisher,
            settings=request.app.state.settings,
            compliance=request.app.state.compliance_client,
            notify=request.app.state.notification_client,
        )


def caller_address(
    request: Request,
    x_creator_addr: str | None = Header(default=None, alias="X-Creator-Addr"),
) -> str:
    """Auth seam (work05): the API gateway injects the authenticated caller address. In dev we fall
    back to the service signer address so the create→cancel ownership check stays consistent."""
    signer_addr: str = request.app.state.signer.address
    return (x_creator_addr or signer_addr).lower()


def caller_user_id(
    x_user_id: str | None = Header(default=None, alias="X-User-Id"),
) -> str | None:
    """Auth seam (work05/work12): the gateway injects the authenticated user id (JWT ``sub``) so the
    compliance gate can evaluate KYC by user. Mirrors the X-Creator-Addr seam; ``None`` in pure-dev
    calls (the gate then skips). Distinct from caller_address (the on-chain address)."""
    return x_user_id


# Common annotated dependencies.
ServiceDep = Depends(get_service)
SettingsDep = Depends(get_settings)
IdempotencyDep = Depends(get_idempotency)
CallerDep = Depends(caller_address)
CallerUserDep = Depends(caller_user_id)
