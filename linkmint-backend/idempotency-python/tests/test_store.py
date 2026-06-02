"""The Redis-backed Idempotency-Key store (fakeredis)."""

from __future__ import annotations

import fakeredis.aioredis
import pytest

from linkmint_idempotency import CachedResponse, IdempotencyConflict, IdempotencyStore, fingerprint


def _store(service: str = "svc") -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), service, 3600)


async def test_first_begin_reserves_then_replays_after_complete() -> None:
    store = _store()
    assert await store.begin("create", "k1", "fp") is None  # reserved, caller owns
    await store.complete("create", "k1", "fp", 201, {"id": "a"})
    cached = await store.begin("create", "k1", "fp")
    assert cached == CachedResponse(http_status=201, body={"id": "a"})


async def test_different_body_is_conflict() -> None:
    store = _store()
    await store.begin("create", "k2", "fp-A")
    with pytest.raises(IdempotencyConflict) as exc:
        await store.begin("create", "k2", "fp-B")
    assert exc.value.reason == "body_mismatch"


async def test_in_flight_duplicate_is_conflict() -> None:
    store = _store()
    await store.begin("create", "k3", "fp")
    with pytest.raises(IdempotencyConflict) as exc:
        await store.begin("create", "k3", "fp")  # still pending → conflict
    assert exc.value.reason == "in_flight"


async def test_release_allows_retry() -> None:
    store = _store()
    await store.begin("create", "k4", "fp")
    await store.release("create", "k4")
    assert await store.begin("create", "k4", "fp") is None  # fresh again


async def test_routes_do_not_collide() -> None:
    store = _store()
    await store.begin("create", "k5", "fp")
    assert await store.begin("cancel", "k5", "fp") is None  # different route owns its own key


def test_key_includes_service() -> None:
    assert _store("svc-a")._key("create", "k") == "idem:svc-a:create:k"
    assert _store("svc-a")._key("create", "k") != _store("svc-b")._key("create", "k")


def test_fingerprint_is_order_insensitive() -> None:
    assert fingerprint({"a": 1, "b": 2}) == fingerprint({"b": 2, "a": 1})
    assert fingerprint({"a": 1}) != fingerprint({"a": 2})
