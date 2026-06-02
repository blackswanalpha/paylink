"""Consumer dedupe helpers: RedisDedupe (fakeredis) + DbDedupe (fake executor)."""

from __future__ import annotations

import fakeredis.aioredis
import pytest

from linkmint_idempotency import DbDedupe, RedisDedupe


def _dedupe() -> RedisDedupe:
    return RedisDedupe(fakeredis.aioredis.FakeRedis(decode_responses=True), "svc", 3600)


async def test_redis_runs_once_then_skips() -> None:
    dd = _dedupe()
    calls = 0

    async def action() -> str:
        nonlocal calls
        calls += 1
        return "done"

    assert await dd.run_once("proof", "h1", action) == "done"
    assert await dd.run_once("proof", "h1", action) is None  # duplicate → skipped
    assert calls == 1


async def test_redis_error_rolls_back_marker() -> None:
    dd = _dedupe()

    async def boom() -> None:
        raise RuntimeError("boom")

    with pytest.raises(RuntimeError):
        await dd.run_once("proof", "h1", boom)

    # Marker rolled back → a redelivery retries (action runs again, cleanly this time).
    calls = 0

    async def ok() -> str:
        nonlocal calls
        calls += 1
        return "ok"

    assert await dd.run_once("proof", "h1", ok) == "ok"
    assert calls == 1


async def test_redis_seen_before() -> None:
    dd = _dedupe()
    assert await dd.seen_before("proof", "h1") is False

    async def noop() -> None:
        return None

    await dd.run_once("proof", "h1", noop)
    assert await dd.seen_before("proof", "h1") is True


def test_redis_key_includes_service_and_scope() -> None:
    dd = _dedupe()
    assert dd._key("proof", "h1") == "idemc:svc:proof:h1"


# --- DbDedupe against an in-memory fake executor (emulates ON CONFLICT DO NOTHING) ---


class _FakeResult:
    def __init__(self, rowcount: int) -> None:
        self.rowcount = rowcount


class _FakeConn:
    def __init__(self) -> None:
        self.seen: set[tuple[str, str]] = set()

    async def execute(self, statement: object, parameters: dict[str, str]) -> _FakeResult:
        key = (parameters["scope"], parameters["key"])
        if key in self.seen:
            return _FakeResult(0)
        self.seen.add(key)
        return _FakeResult(1)


async def test_db_runs_once_then_skips() -> None:
    dd = DbDedupe()
    conn = _FakeConn()
    calls = 0

    async def action() -> str:
        nonlocal calls
        calls += 1
        return "done"

    ran, result = await dd.run_once(conn, "proof", "h1", action)
    assert ran is True and result == "done"
    ran, result = await dd.run_once(conn, "proof", "h1", action)
    assert ran is False and result is None  # duplicate → action skipped
    assert calls == 1


def test_db_rejects_unsafe_table() -> None:
    assert DbDedupe("bad; DROP TABLE x")._table == "processed_events"
    assert DbDedupe("svc.processed_events")._table == "svc.processed_events"
    assert DbDedupe()._table == "processed_events"
