from __future__ import annotations

import fakeredis.aioredis
import pytest

from app.errors import AppError, ErrorCode
from app.idempotency import IdempotencyStore, fingerprint


@pytest.fixture
def store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


async def test_first_call_reserves(store: IdempotencyStore) -> None:
    assert await store.begin("create", "k1", "fp1") is None


async def test_replay_returns_cached_response(store: IdempotencyStore) -> None:
    await store.begin("create", "k1", "fp1")
    await store.complete("create", "k1", "fp1", 201, {"pl_id": "0x1"})
    cached = await store.begin("create", "k1", "fp1")
    assert cached is not None
    assert cached.http_status == 201
    assert cached.body == {"pl_id": "0x1"}


async def test_different_body_conflicts(store: IdempotencyStore) -> None:
    await store.begin("create", "k1", "fp1")
    await store.complete("create", "k1", "fp1", 201, {})
    with pytest.raises(AppError) as exc:
        await store.begin("create", "k1", "fp2")
    assert exc.value.code is ErrorCode.IDEMPOTENT_CONFLICT


async def test_in_flight_duplicate_conflicts(store: IdempotencyStore) -> None:
    await store.begin("create", "k1", "fp1")  # reserved, not completed
    with pytest.raises(AppError) as exc:
        await store.begin("create", "k1", "fp1")
    assert exc.value.code is ErrorCode.IDEMPOTENT_CONFLICT


async def test_release_allows_retry(store: IdempotencyStore) -> None:
    await store.begin("create", "k1", "fp1")
    await store.release("create", "k1")
    assert await store.begin("create", "k1", "fp1") is None


async def test_routes_are_namespaced(store: IdempotencyStore) -> None:
    assert await store.begin("create", "k1", "fp1") is None
    # same key on a different route does not collide
    assert await store.begin("cancel", "k1", "fp1") is None


def test_fingerprint_is_order_independent() -> None:
    assert fingerprint({"a": 1, "b": 2}) == fingerprint({"b": 2, "a": 1})
    assert fingerprint({"a": 1}) != fingerprint({"a": 2})
