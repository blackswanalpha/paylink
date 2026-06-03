"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager, suppress

import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from linkmint_idempotency import IdempotencyStore
from linkmint_telemetry import ObservabilityMiddleware, init_telemetry
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.auth import router as auth_router
from app.api.v1.internal_admin import router as internal_admin_router
from app.api.v1.organizations import router as organizations_router
from app.api.v1.sessions import router as sessions_router
from app.api.v1.users import router as users_router
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.events.stub import build_publisher
from app.logging import RequestIdMiddleware, configure_logging, get_logger
from app.security.jwt import JwtIssuer, JwtVerifier
from app.security.keys import KeyStore
from app.security.login_attempts import FailedLoginCounter
from app.security.mfa_crypto import MfaCipher
from app.security.oauth.registry import build_oauth_resolver
from app.security.passwords import PasswordHashing


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
    app.state.publisher = build_publisher(settings)

    # work15 — outbox-drain relay (drains the identity_events outbox to the bus). Lazily imported so
    # default "log" mode needs no broker / linkmint_eventbus, keeping existing tests broker-free.
    app.state.relay_task = None
    if settings.event_publisher_mode == "kafka":
        from linkmint_eventbus import KafkaPublisher

        from app.events.relay import OutboxRelay

        relay = OutboxRelay(
            app.state.sessionmaker,
            KafkaPublisher(settings.kafka_broker_list, source=settings.service_name),
            schema="identity",
            table="identity_events",
            key_column="subject_id",
        )
        app.state.relay_task = asyncio.create_task(relay.run())
        log.info("outbox_relay_started", brokers=settings.kafka_broker_list)

    keys = KeyStore.from_settings(settings)
    app.state.keys = keys
    app.state.jwt_issuer = JwtIssuer(
        keys,
        issuer=settings.jwt_issuer,
        audience=settings.jwt_audience,
        ttl_seconds=settings.access_token_ttl_seconds,
    )
    app.state.jwt_verifier = JwtVerifier(
        keys.public_pem, issuer=settings.jwt_issuer, audience=settings.jwt_audience
    )
    app.state.passwords = PasswordHashing.from_settings(settings)
    app.state.mfa_cipher = MfaCipher.from_settings(settings)
    app.state.oauth_resolver = build_oauth_resolver(settings)
    app.state.failed_login = FailedLoginCounter(app.state.redis)

    log.info(
        "startup",
        jwt_issuer=settings.jwt_issuer,
        jwt_kid=keys.kid,
        oauth_fake=settings.oauth_fake,
    )
    if settings.password_reset_dev_return_token:
        # DEV ONLY: the reset-request response echoes the raw reset token. Never enable in prod —
        # log loudly so an accidental enablement is impossible to miss.
        log.warning(
            "password_reset_dev_return_token_enabled",
            detail="raw reset tokens are returned in API responses — DEV/LOCAL ONLY",
        )
    # work15 — bus consumer (subscribes to the bus → KycConsumer.handle). Lazily imported; runs only
    # when IDENTITY_EVENT_CONSUMER_ENABLED=true (all app.state is set by now).
    app.state.bus_task = None
    if settings.event_consumer_enabled:
        from app.busconsumer.run import run as run_bus_consumer

        app.state.bus_task = asyncio.create_task(run_bus_consumer(app))
        log.info("bus_consumer_enabled", brokers=settings.kafka_broker_list)

    try:
        yield
    finally:
        if app.state.relay_task is not None:
            app.state.relay_task.cancel()
            with suppress(asyncio.CancelledError):
                await app.state.relay_task
        if app.state.bus_task is not None:
            app.state.bus_task.cancel()
            with suppress(asyncio.CancelledError):
                await app.state.bus_task
        await app.state.redis.aclose()
        await engine.dispose()
        app.state.otel_shutdown()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="identity-service", version="0.1.0", lifespan=lifespan)
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
    app.include_router(auth_router)
    app.include_router(users_router)
    app.include_router(organizations_router)
    app.include_router(sessions_router)
    app.include_router(internal_admin_router)

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
        return JSONResponse(
            status_code=200 if ready else 503,
            content={"status": "ready" if ready else "not_ready", "checks": checks},
        )

    app.mount("/metrics", make_asgi_app())
    return app


app = create_app()
