"""The Redis-backed Idempotency-Key store (fakeredis)."""

from __future__ import annotations

import fakeredis.aioredis
import pytest

from app.errors import AppError, ErrorCode
from app.idempotency import IdempotencyStore


def _store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


async def test_first_begin_reserves_then_replays_after_complete() -> None:
    store = _store()
    assert await store.begin("notifications", "k1", "fp") is None  # reserved, caller owns
    await store.complete("notifications", "k1", "fp", 201, {"delivery_ids": ["a"]})
    cached = await store.begin("notifications", "k1", "fp")
    assert cached is not None
    assert cached.http_status == 201
    assert cached.body == {"delivery_ids": ["a"]}


async def test_different_body_is_conflict() -> None:
    store = _store()
    await store.begin("notifications", "k2", "fp-A")
    with pytest.raises(AppError) as exc:
        await store.begin("notifications", "k2", "fp-B")
    assert exc.value.code == ErrorCode.IDEMPOTENT_CONFLICT


async def test_in_flight_duplicate_is_conflict() -> None:
    store = _store()
    await store.begin("notifications", "k3", "fp")
    with pytest.raises(AppError):
        await store.begin("notifications", "k3", "fp")  # still pending → conflict


async def test_release_allows_retry() -> None:
    store = _store()
    await store.begin("notifications", "k4", "fp")
    await store.release("notifications", "k4")
    assert await store.begin("notifications", "k4", "fp") is None  # fresh again
