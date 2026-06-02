"""Integration: DbDedupe against real Postgres; store + RedisDedupe against real Redis."""

from __future__ import annotations

from typing import Any

import pytest
from sqlalchemy.ext.asyncio import AsyncEngine

from linkmint_idempotency import DbDedupe, IdempotencyConflict, IdempotencyStore, RedisDedupe

pytestmark = pytest.mark.integration


async def test_db_dedupe_exactly_once(engine: AsyncEngine) -> None:
    dd = DbDedupe("processed_events")
    calls = 0

    async def action() -> None:
        nonlocal calls
        calls += 1

    # Each delivery handled in its OWN transaction (mirrors a consumer handling one event per tx).
    for _ in range(3):
        async with engine.begin() as conn:
            await dd.run_once(conn, "proof", "h1", action)
    assert calls == 1


async def test_store_lifecycle_real_redis(redis_client: Any) -> None:
    store = IdempotencyStore(redis_client, "svc", 3600)
    assert await store.begin("create", "k1", "fp1") is None
    await store.complete("create", "k1", "fp1", 201, {"id": "p1"})
    cached = await store.begin("create", "k1", "fp1")
    assert cached is not None and cached.http_status == 201
    with pytest.raises(IdempotencyConflict):  # same key, different body
        await store.begin("create", "k1", "fp2")


async def test_redis_dedupe_real_redis(redis_client: Any) -> None:
    dd = RedisDedupe(redis_client, "svc", 3600)
    calls = 0

    async def action() -> None:
        nonlocal calls
        calls += 1

    for _ in range(3):
        await dd.run_once("proof", "h1", action)
    assert calls == 1
