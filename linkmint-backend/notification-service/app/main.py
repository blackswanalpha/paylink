"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

import uuid
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import httpx
import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.internal import router as internal_router
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.idempotency import IdempotencyStore
from app.logging import RequestIdMiddleware, configure_logging, get_logger
from app.recipients.base import RecipientResolver
from app.recipients.identity import IdentityRecipientResolver
from app.recipients.inline import InlineRecipientResolver


def _build_resolver(settings: Settings, client: httpx.AsyncClient) -> RecipientResolver:
    if settings.recipient_resolver == "identity":
        token = (
            settings.identity_internal_token.get_secret_value()
            if settings.identity_internal_token
            else None
        )
        return IdentityRecipientResolver(
            client, base_url=settings.identity_service_url, internal_token=token
        )
    return InlineRecipientResolver()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings: Settings = app.state.settings
    configure_logging(settings.log_level, settings.service_name)
    log = get_logger()

    # Importing the task here (not at module load) keeps the import graph lean and lets tests that
    # never enqueue avoid constructing the Celery app.
    from app.celeryapp.tasks import deliver

    engine = make_engine(settings.database_url)
    app.state.engine = engine
    app.state.sessionmaker = make_sessionmaker(engine)
    app.state.redis = aioredis.from_url(settings.redis_url, decode_responses=True)
    app.state.broker_redis = aioredis.from_url(settings.celery_broker_url, decode_responses=True)
    app.state.idempotency = IdempotencyStore(app.state.redis, settings.idempotency_ttl_seconds)
    app.state.http_client = httpx.AsyncClient(timeout=10.0)
    app.state.recipient_resolver = _build_resolver(settings, app.state.http_client)

    def _enqueue(delivery_id: uuid.UUID) -> None:
        # Best-effort dispatch AFTER the row is durably committed: a broker hiccup (or, in eager
        # dev/test mode, a task that exhausts its retries inline) must not 500 the caller or skip
        # the remaining deliveries — the row stays QUEUED and is recoverable.
        try:
            deliver.apply_async(args=(str(delivery_id),), queue="notify")
        except Exception as exc:  # noqa: BLE001 - enqueue is best-effort; row is durably QUEUED
            log.error("enqueue_failed", delivery_id=str(delivery_id), error=str(exc))

    app.state.enqueue = _enqueue

    log.info(
        "startup",
        sms_provider=settings.sms_provider,
        email_provider=settings.email_provider,
        recipient_resolver=settings.recipient_resolver,
        internal_token_required=settings.internal_shared_secret is not None,
        celery_eager=settings.celery_task_always_eager,
    )
    try:
        yield
    finally:
        await app.state.http_client.aclose()
        await app.state.redis.aclose()
        await app.state.broker_redis.aclose()
        await engine.dispose()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="notification-service", version="0.1.0", lifespan=lifespan)
    app.state.settings = settings or get_settings()
    app.add_middleware(RequestIdMiddleware)
    install_error_handlers(app)
    app.include_router(internal_router)

    @app.get("/internal/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/internal/readyz")
    async def readyz() -> JSONResponse:
        checks: dict[str, str] = {}
        ready = True
        try:
            async with app.state.sessionmaker() as session:
                await session.execute(text("SELECT 1"))
            checks["db"] = "ok"
        except Exception:  # noqa: BLE001 - readiness probe reports, never raises
            checks["db"] = "error"
            ready = False
        try:
            await app.state.redis.ping()
            checks["redis"] = "ok"
        except Exception:  # noqa: BLE001
            checks["redis"] = "error"
            ready = False
        # The broker is a soft dependency: the API can accept + persist deliveries even if the
        # broker is briefly down (the worker drains on recovery), so a broker blip degrades.
        try:
            await app.state.broker_redis.ping()
            checks["broker"] = "ok"
        except Exception:  # noqa: BLE001
            checks["broker"] = "degraded"

        return JSONResponse(
            status_code=200 if ready else 503,
            content={"status": "ready" if ready else "not_ready", "checks": checks},
        )

    app.mount("/metrics", make_asgi_app())
    return app


app = create_app()
