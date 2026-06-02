"""Integration fixtures: real Postgres via testcontainers (skipped without Docker).

Applies the canonical ledger migration (the same SQL the Go one-shot migrator runs) to a fresh
postgres:16, then hands tests an async engine. Each test starts from an empty table (TRUNCATE is
allowed — the append-only trigger only blocks UPDATE/DELETE).
"""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from pathlib import Path

import pytest
import pytest_asyncio
import sqlalchemy as sa
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine

MIGRATION = (
    Path(__file__).resolve().parents[2] / "src" / "linkmint_ledger" / "migrations" / "0001_init.sql"
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
    # Apply the schema via the raw DBAPI (multi-statement script incl. the plpgsql trigger function).
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
        await conn.exec_driver_sql("TRUNCATE ledger.ledger_entries RESTART IDENTITY")
    yield eng
    await eng.dispose()
