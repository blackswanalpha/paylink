"""Integration fixtures: real Postgres + Redis via testcontainers (skipped without Docker).

Applies the canonical processed_events migration (the same SQL the Go services run) to a fresh
postgres:16, then hands tests an async engine; also boots redis:7 and hands tests an async client.
"""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from pathlib import Path
from typing import Any

import pytest
import pytest_asyncio
import sqlalchemy as sa
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine

MIGRATION = (
    Path(__file__).resolve().parents[2]
    / "src"
    / "linkmint_idempotency"
    / "migrations"
    / "processed_events.sql"
)


def _psycopg_url(raw: str) -> str:
    if "+psycopg2" in raw:
        return raw.replace("+psycopg2", "+psycopg")
    if "+psycopg" not in raw:
        return raw.replace("postgresql://", "postgresql+psycopg://", 1)
    return raw


@pytest.fixture(scope="session")
def pg_url() -> Iterator[str]:
    pg_mod = pytest.importorskip("testcontainers.postgres")
    try:
        container = pg_mod.PostgresContainer("postgres:16")
        container.start()
    except Exception as exc:  # noqa: BLE001 - any Docker failure → skip the integration suite
        pytest.skip(f"Docker/Postgres unavailable: {exc}")

    url = _psycopg_url(container.get_connection_url())
    engine = sa.create_engine(url)
    with engine.begin() as conn:
        conn.exec_driver_sql(MIGRATION.read_text())
    engine.dispose()
    try:
        yield url
    finally:
        container.stop()


@pytest_asyncio.fixture
async def engine(pg_url: str) -> AsyncIterator[AsyncEngine]:
    eng = create_async_engine(pg_url)
    async with eng.begin() as conn:
        await conn.exec_driver_sql("TRUNCATE processed_events")
    yield eng
    await eng.dispose()


@pytest_asyncio.fixture
async def redis_client() -> AsyncIterator[Any]:
    redis_mod = pytest.importorskip("testcontainers.redis")
    aioredis = pytest.importorskip("redis.asyncio")
    try:
        container = redis_mod.RedisContainer("redis:7")
        container.start()
    except Exception as exc:  # noqa: BLE001 - any Docker failure → skip
        pytest.skip(f"Docker/Redis unavailable: {exc}")

    host = container.get_container_host_ip()
    port = container.get_exposed_port(6379)
    client = aioredis.from_url(f"redis://{host}:{port}/0", decode_responses=True)
    try:
        yield client
    finally:
        await client.aclose()
        container.stop()
