"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager, suppress

import redis.asyncio as aioredis
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from linkmint_idempotency import IdempotencyStore
from linkmint_telemetry import ObservabilityMiddleware, init_telemetry, traced_async_client
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.compliance import router as compliance_router
from app.api.v1.kyc import router as kyc_router
from app.api.v1.risk import router as risk_router
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.events.stub import build_publisher
from app.logging import RequestIdMiddleware, configure_logging, get_logger
from app.providers.registry import build_registry
from app.security.jwt import JwtVerifier
from app.security.provider_crypto import ProviderCipher


def _resolve_jwt_public_pem(settings: Settings) -> str:
    """Return identity-service's RS256 public PEM, or an ephemeral one for zero-config dev.

    compliance-risk is a verifier-only consumer; without the real public key NO token verifies, so
    when unset we generate an ephemeral public key purely so the verifier constructs and the app
    boots — and log a warning. Tests inject the matching public key.
    """
    if settings.jwt_public_key_pem:
        return settings.jwt_public_key_pem
    ephemeral = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    pem = (
        ephemeral.public_key()
        .public_bytes(
            serialization.Encoding.PEM,
            serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        .decode()
    )
    get_logger().warning(
        "jwt_public_key_unset",
        detail="COMPLIANCE_JWT_PUBLIC_KEY_PEM is unset; generated an ephemeral public key — "
        "NO real identity token will verify. Set identity-service's public key in prod.",
    )
    return pem


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

    # work15 — outbox-drain relay (drains the compliance_events outbox to the bus). Lazily imported
    # so default "log" mode needs no broker / linkmint_eventbus, keeping existing tests broker-free.
    app.state.relay_task = None
    if settings.event_publisher_mode == "kafka":
        from linkmint_eventbus import KafkaPublisher

        from app.events.relay import OutboxRelay

        relay = OutboxRelay(
            app.state.sessionmaker,
            KafkaPublisher(settings.kafka_broker_list, source=settings.service_name),
            schema="compliance",
            table="compliance_events",
            key_column="subject_id",
        )
        app.state.relay_task = asyncio.create_task(relay.run())
        log.info("outbox_relay_started", brokers=settings.kafka_broker_list)

    # work18 — outbound client injects W3C trace context onto provider/KYC calls.
    app.state.http_client = traced_async_client(timeout=10.0)
    app.state.provider_registry = build_registry(settings, app.state.http_client)
    app.state.provider_cipher = ProviderCipher.from_settings(settings)
    app.state.jwt_verifier = JwtVerifier(
        _resolve_jwt_public_pem(settings),
        issuer=settings.jwt_issuer,
        audience=settings.jwt_audience,
    )

    log.info(
        "startup",
        jwt_issuer=settings.jwt_issuer,
        jwt_audience=settings.jwt_audience,
        kyc_provider=settings.kyc_provider,
        internal_token_required=settings.internal_shared_secret is not None,
    )
    # work15 — bus consumer (subscribes to the bus → the handle() chokepoint). Lazily imported;
    # runs only when COMPLIANCE_EVENT_CONSUMER_ENABLED=true (all app.state is set by now).
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
        await app.state.http_client.aclose()
        await app.state.redis.aclose()
        await engine.dispose()
        app.state.otel_shutdown()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="compliance-risk", version="0.1.0", lifespan=lifespan)
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
    app.include_router(kyc_router)
    app.include_router(compliance_router)
    app.include_router(risk_router)

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
