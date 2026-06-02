"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager, suppress

import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from linkmint_idempotency import IdempotencyStore
from linkmint_telemetry import ObservabilityMiddleware, init_telemetry, traced_async_client
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.paylinks import router as paylinks_router
from app.chain.client import ChainClient
from app.chain.nonce import NonceManager
from app.chain.signer import build_signer
from app.compliance.client import ComplianceClient
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.events.stub import build_publisher
from app.logging import RequestIdMiddleware, configure_logging, get_logger
from app.notifications.client import NotificationClient


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    settings: Settings = app.state.settings
    configure_logging(settings.log_level, settings.service_name)
    log = get_logger()
    # work18 — OpenTelemetry tracing. A no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set.
    app.state.otel_shutdown = init_telemetry(settings.service_name, "0.1.0")

    engine = make_engine(settings.database_url)
    app.state.engine = engine
    app.state.sessionmaker = make_sessionmaker(engine)
    app.state.redis = aioredis.from_url(settings.redis_url, decode_responses=True)
    app.state.idempotency = IdempotencyStore(
        app.state.redis, settings.service_name, settings.idempotency_ttl_seconds
    )
    # work18 — the outbound client injects W3C trace context, so the HTTP hop to compliance-risk /
    # notification-service continues this request's trace.
    app.state.http = traced_async_client(timeout=10.0)
    app.state.chain_client = ChainClient(settings.chain_rpc_url, app.state.http)
    app.state.signer = build_signer(settings)
    app.state.nonces = NonceManager(app.state.chain_client)
    app.state.publisher = build_publisher(settings)

    # work15 — outbox-drain relay (drains paylink.paylink_events to the bus). Lazily imported so the
    # default "log" mode needs no broker / linkmint_eventbus, keeping existing tests broker-free.
    app.state.relay_task = None
    if settings.event_publisher_mode == "kafka":
        from linkmint_eventbus import KafkaPublisher

        from app.events.relay import OutboxRelay

        relay = OutboxRelay(
            app.state.sessionmaker,
            KafkaPublisher(settings.kafka_broker_list, source=settings.service_name),
            schema="paylink",
            table="paylink_events",
            key_column="pl_id",
        )
        app.state.relay_task = asyncio.create_task(relay.run())
        log.info("outbox_relay_started", brokers=settings.kafka_broker_list)

    app.state.compliance_client = (
        ComplianceClient(
            settings.compliance_service_url,
            app.state.http,
            internal_token=(
                settings.compliance_internal_token.get_secret_value()
                if settings.compliance_internal_token
                else None
            ),
            timeout=settings.compliance_timeout_seconds,
        )
        if settings.compliance_check_enabled
        else None
    )

    app.state.notification_client = (
        NotificationClient(
            settings.notify_service_url,
            app.state.http,
            internal_token=(
                settings.notify_internal_token.get_secret_value()
                if settings.notify_internal_token
                else None
            ),
            timeout=settings.notify_timeout_seconds,
        )
        if settings.notify_enabled
        else None
    )

    log.info(
        "startup",
        signer_address=app.state.signer.address,
        chain_rpc=settings.chain_rpc_url,
        chain_submit_enabled=settings.chain_submit_enabled,
        compliance_check_enabled=settings.compliance_check_enabled,
        notify_enabled=settings.notify_enabled,
    )
    try:
        yield
    finally:
        if app.state.relay_task is not None:
            app.state.relay_task.cancel()
            with suppress(asyncio.CancelledError):
                await app.state.relay_task
        await app.state.http.aclose()
        await app.state.redis.aclose()
        await engine.dispose()
        app.state.otel_shutdown()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="paylink-service", version="0.1.0", lifespan=lifespan)
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
    app.include_router(paylinks_router)

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
        # The chain is a soft dependency: reads still work without it, so a chain outage degrades
        # but does not fail readiness.
        try:
            await app.state.chain_client.chain_height()
            checks["chain"] = "ok"
        except Exception:  # noqa: BLE001
            checks["chain"] = "degraded"

        return JSONResponse(
            status_code=200 if ready else 503,
            content={"status": "ready" if ready else "not_ready", "checks": checks},
        )

    app.mount("/metrics", make_asgi_app())
    return app


app = create_app()
