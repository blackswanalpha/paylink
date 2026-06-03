"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

import asyncio
import uuid
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager, suppress

import httpx
import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from linkmint_idempotency import IdempotencyStore
from linkmint_telemetry import (
    ObservabilityMiddleware,
    init_telemetry,
    inject_trace_headers,
    traced_async_client,
)
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.inbox import router as inbox_router
from app.api.v1.internal import router as internal_router
from app.api.v1.preferences import router as preferences_router
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
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
    # work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set.
    app.state.otel_shutdown = init_telemetry(settings.service_name, "0.1.0")

    # Importing the task here (not at module load) keeps the import graph lean and lets tests that
    # never enqueue avoid constructing the Celery app.
    from app.celeryapp.tasks import deliver

    engine = make_engine(settings.database_url)
    app.state.engine = engine
    app.state.sessionmaker = make_sessionmaker(engine)
    app.state.redis = aioredis.from_url(settings.redis_url, decode_responses=True)
    app.state.broker_redis = aioredis.from_url(settings.celery_broker_url, decode_responses=True)
    app.state.idempotency = IdempotencyStore(
        app.state.redis, settings.service_name, settings.idempotency_ttl_seconds
    )
    app.state.http_client = traced_async_client(timeout=10.0)
    app.state.recipient_resolver = _build_resolver(settings, app.state.http_client)

    def _enqueue(delivery_id: uuid.UUID) -> None:
        # Best-effort dispatch AFTER the row is durably committed: a broker hiccup (or, in eager
        # dev/test mode, a task that exhausts its retries inline) must not 500 the caller or skip
        # the remaining deliveries — the row stays QUEUED and is recoverable.
        try:
            # work18 — carry the trace context into the Celery message so the worker continues this
            # request's trace (paired with worker_span in app/celeryapp/tasks.py).
            deliver.apply_async(
                args=(str(delivery_id),), queue="notify", headers=inject_trace_headers()
            )
        except Exception as exc:  # noqa: BLE001 - enqueue is best-effort; row is durably QUEUED
            log.error("enqueue_failed", delivery_id=str(delivery_id), error=str(exc))

    app.state.enqueue = _enqueue

    # work15 — bus consumer (subscribes to paylink/payment topics → the handle() chokepoint).
    # Lazily imported so the default (disabled) path needs no broker / linkmint_eventbus.
    app.state.bus_task = None
    if settings.event_consumer_enabled:
        from app.busconsumer.run import run as run_bus_consumer

        app.state.bus_task = asyncio.create_task(run_bus_consumer(app))
        log.info("bus_consumer_enabled", brokers=settings.kafka_broker_list)

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
        if app.state.bus_task is not None:
            app.state.bus_task.cancel()
            with suppress(asyncio.CancelledError):
                await app.state.bus_task
        await app.state.http_client.aclose()
        await app.state.redis.aclose()
        await app.state.broker_redis.aclose()
        await engine.dispose()
        app.state.otel_shutdown()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="notification-service", version="0.1.0", lifespan=lifespan)
    app.state.settings = settings or get_settings()
    app.add_middleware(RequestIdMiddleware)
    # work18 — added after RequestIdMiddleware so it wraps it (outermost): starts the server span
    # and seeds X-Request-Id with the trace id, which RequestIdMiddleware then adopts as the
    # correlation id (one id across logs, the envelope, the response header, and Tempo).
    app.add_middleware(
        ObservabilityMiddleware,
        service_name=app.state.settings.service_name,
        routes=app.routes,
    )
    install_error_handlers(app)
    app.include_router(internal_router)
    app.include_router(inbox_router)
    app.include_router(preferences_router)

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
