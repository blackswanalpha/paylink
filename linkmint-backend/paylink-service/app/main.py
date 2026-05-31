"""FastAPI application factory + lifespan wiring."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import httpx
import redis.asyncio as aioredis
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from prometheus_client import make_asgi_app
from sqlalchemy import text

from app.api.v1.paylinks import router as paylinks_router
from app.chain.client import ChainClient
from app.chain.nonce import NonceManager
from app.chain.signer import build_signer
from app.config import Settings, get_settings
from app.db.session import make_engine, make_sessionmaker
from app.errors import install_error_handlers
from app.events.stub import build_publisher
from app.idempotency import IdempotencyStore
from app.logging import RequestIdMiddleware, configure_logging, get_logger


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
    app.state.http = httpx.AsyncClient(timeout=10.0)
    app.state.chain_client = ChainClient(settings.chain_rpc_url, app.state.http)
    app.state.signer = build_signer(settings)
    app.state.nonces = NonceManager(app.state.chain_client)
    app.state.publisher = build_publisher(settings)

    log.info(
        "startup",
        signer_address=app.state.signer.address,
        chain_rpc=settings.chain_rpc_url,
        chain_submit_enabled=settings.chain_submit_enabled,
    )
    try:
        yield
    finally:
        await app.state.http.aclose()
        await app.state.redis.aclose()
        await engine.dispose()
        log.info("shutdown")


def create_app(settings: Settings | None = None) -> FastAPI:
    app = FastAPI(title="paylink-service", version="0.1.0", lifespan=lifespan)
    app.state.settings = settings or get_settings()
    app.add_middleware(RequestIdMiddleware)
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
