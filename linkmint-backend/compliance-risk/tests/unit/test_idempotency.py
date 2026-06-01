from __future__ import annotations

import fakeredis.aioredis
import pytest

from app.errors import AppError, ErrorCode
from app.idempotency import IdempotencyStore, fingerprint


def _store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


async def test_owner_then_replay() -> None:
    s = _store()
    fp = fingerprint({"a": 1})
    assert await s.begin("r", "k1", fp) is None
    await s.complete("r", "k1", fp, 201, {"ok": True})
    cached = await s.begin("r", "k1", fp)
    assert cached is not None
    assert cached.http_status == 201
    assert cached.body == {"ok": True}


async def test_body_mismatch_is_conflict() -> None:
    s = _store()
    await s.begin("r", "k", fingerprint({"a": 1}))
    with pytest.raises(AppError) as exc:
        await s.begin("r", "k", fingerprint({"a": 2}))
    assert exc.value.code == ErrorCode.IDEMPOTENT_CONFLICT


async def test_in_flight_duplicate_is_conflict() -> None:
    s = _store()
    fp = fingerprint({"a": 1})
    await s.begin("r", "k", fp)
    with pytest.raises(AppError):
        await s.begin("r", "k", fp)


async def test_release_allows_retry() -> None:
    s = _store()
    fp = fingerprint({"a": 1})
    await s.begin("r", "k", fp)
    await s.release("r", "k")
    assert await s.begin("r", "k", fp) is None


def test_namespace_is_compliance_scoped() -> None:
    assert IdempotencyStore._key("create_session", "k") == "idem:compliance-risk:create_session:k"


def test_fingerprint_is_order_independent() -> None:
    assert fingerprint({"a": 1, "b": 2}) == fingerprint({"b": 2, "a": 1})
