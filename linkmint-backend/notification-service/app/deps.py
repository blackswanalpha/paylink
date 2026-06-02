"""FastAPI dependencies. Singletons (redis, sessionmaker, recipient resolver, the Celery enqueue
callable) live on ``app.state`` (wired in :mod:`app.main`); each request gets a fresh DB session.

The intake surface (``POST /v1/notifications``) is the work15 *bus stand-in* — service-to-service,
no per-request JWT. :func:`internal_gate` hardens it: when ``NOTIFY_INTERNAL_SHARED_SECRET`` is set
it requires a constant-time ``X-Internal-Token`` match; when unset, the network is the only control
(ADR-009 / compliance + audit-log precedent).
"""

from __future__ import annotations

import hmac
from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Header, Request
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.db.repository import NotifyRepository
from app.domain.service import NotificationService
from app.errors import AppError, ErrorCode
from app.events.consumer import NotificationEventConsumer
from app.templating.registry import TemplateRegistry


def get_settings(request: Request) -> Settings:
    settings: Settings = request.app.state.settings
    return settings


def get_idempotency(request: Request) -> IdempotencyStore:
    store: IdempotencyStore = request.app.state.idempotency
    return store


async def get_repo(request: Request) -> AsyncIterator[NotifyRepository]:
    """A read repository over a fresh async session (for GET /internal/deliveries/{id})."""
    sessionmaker = request.app.state.sessionmaker
    async with sessionmaker() as session:
        yield NotifyRepository(session)


async def get_consumer(request: Request) -> AsyncIterator[NotificationEventConsumer]:
    """The event chokepoint for this request: a NotificationService over a fresh session, wrapped in
    the same ``NotificationEventConsumer.handle`` the future work15 bus subscriber will call."""
    sessionmaker = request.app.state.sessionmaker
    async with sessionmaker() as session:
        repo = NotifyRepository(session)
        service = NotificationService(
            repo=repo,
            registry=TemplateRegistry(repo),
            resolver=request.app.state.recipient_resolver,
            enqueue=request.app.state.enqueue,
            commit=session.commit,
        )
        yield NotificationEventConsumer(service)


def caller_address(
    request: Request,
    x_creator_addr: str | None = Header(default=None, alias="X-Creator-Addr"),
) -> str:
    """Authenticated caller for the in-app inbox (FE work07), scoped per creator address.

    The API gateway verifies the JWT and injects ``X-Creator-Addr`` (the same seam paylink-service
    trusts). In local direct dev (no gateway) the optional ``NOTIFY_DEV_CREATOR_ADDR`` fallback
    applies; if neither is present the read API is unauthenticated → 401. Lowercased so it matches
    the address paylink-service stores + emits.
    """
    settings: Settings = request.app.state.settings
    addr = x_creator_addr or settings.dev_creator_addr
    if not addr:
        raise AppError(ErrorCode.UNAUTHORIZED, "missing X-Creator-Addr")
    return addr.lower()


def internal_gate(
    request: Request,
    x_internal_token: str | None = Header(default=None),
) -> None:
    """Guard the internal intake surface (trusted network + optional shared secret).

    When ``NOTIFY_INTERNAL_SHARED_SECRET`` is configured, a constant-time ``X-Internal-Token`` match
    is required; when unset, the request is allowed (the deployment's network is the control).
    """
    settings: Settings = request.app.state.settings
    secret = settings.internal_shared_secret
    if secret is None:
        return
    expected = secret.get_secret_value()
    if not x_internal_token or not hmac.compare_digest(x_internal_token, expected):
        raise AppError(ErrorCode.UNAUTHORIZED, "invalid or missing X-Internal-Token")


# Common annotated dependencies.
SettingsDep = Annotated[Settings, Depends(get_settings)]
IdempotencyDep = Annotated[IdempotencyStore, Depends(get_idempotency)]
RepoDep = Annotated[NotifyRepository, Depends(get_repo)]
ConsumerDep = Annotated[NotificationEventConsumer, Depends(get_consumer)]
InternalGateDep = Annotated[None, Depends(internal_gate)]
CallerDep = Annotated[str, Depends(caller_address)]
IdemKey = Annotated[str | None, Header(alias="Idempotency-Key")]
