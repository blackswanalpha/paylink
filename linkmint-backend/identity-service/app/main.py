"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.auth import router as auth_router
from app.api.v1.organizations import router as organizations_router
from app.api.v1.sessions import router as sessions_router
from app.api.v1.users import router as users_router
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.events.stub import build_publisher
from app.idempotency import IdempotencyStore
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

    engine = make_engine(settings.database_url)
    app.state.engine = engine
    app.state.sessionmaker = make_sessionmaker(engine)
    app.state.redis = aioredis.from_url(settings.redis_url, decode_responses=True)
    app.state.idempotency = IdempotencyStore(app.state.redis, settings.idempotency_ttl_seconds)
    app.state.publisher = build_publisher(settings)

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
    try:
        yield
    finally:
        await app.state.redis.aclose()
        await engine.dispose()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="identity-service", version="0.1.0", lifespan=lifespan)
    app.state.settings = settings or get_settings()
    app.add_middleware(RequestIdMiddleware)
    install_error_handlers(app)
    app.include_router(auth_router)
    app.include_router(users_router)
    app.include_router(organizations_router)
    app.include_router(sessions_router)

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
