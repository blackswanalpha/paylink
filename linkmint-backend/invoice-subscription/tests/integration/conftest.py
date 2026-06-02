"""Integration fixtures: real Postgres + Redis via testcontainers (skipped without Docker)."""

from __future__ import annotations

import os
from collections.abc import Iterator
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from app.main import create_app
from tests._support import make_settings

SERVICE_ROOT = Path(__file__).resolve().parents[2]


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
    os.environ["INVOICE_DATABASE_URL"] = url
    from alembic.config import Config

    from alembic import command

    cfg = Config(str(SERVICE_ROOT / "alembic.ini"))
    cfg.set_main_option("script_location", str(SERVICE_ROOT / "alembic"))
    command.upgrade(cfg, "head")
    try:
        yield url
    finally:
        container.stop()
        os.environ.pop("INVOICE_DATABASE_URL", None)


@pytest.fixture(scope="session")
def redis_url() -> Iterator[str]:
    redis_mod = pytest.importorskip("testcontainers.redis")
    try:
        container = redis_mod.RedisContainer("redis:7")
        container.start()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Docker/Redis unavailable: {exc}")
    host = container.get_container_host_ip()
    port = container.get_exposed_port(6379)
    try:
        yield f"redis://{host}:{port}/0"
    finally:
        container.stop()


@pytest.fixture
def live_client(pg_url: str, redis_url: str) -> Iterator[TestClient]:
    """A TestClient over the REAL Postgres + Redis (no dependency overrides)."""
    settings = make_settings(database_url=pg_url, redis_url=redis_url)
    app = create_app(settings)
    with TestClient(app) as client:
        yield client
