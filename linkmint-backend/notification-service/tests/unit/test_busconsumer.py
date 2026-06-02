"""The bus consumer's work17 RedisDedupe short-circuit: a redelivered event is processed once."""

from __future__ import annotations

import types
from typing import Any

import fakeredis.aioredis

from app.busconsumer import run as busrun
from tests._support import EnqueueSpy, make_settings


class _FakeSession:
    async def __aenter__(self) -> _FakeSession:
        return self

    async def __aexit__(self, *_: Any) -> bool:
        return False

    async def commit(self) -> None:
        return None


def _fake_app() -> Any:
    return types.SimpleNamespace(
        state=types.SimpleNamespace(
            settings=make_settings(),
            redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
            sessionmaker=lambda: _FakeSession(),
            recipient_resolver=object(),
            enqueue=EnqueueSpy(),
        )
    )


async def test_redelivered_event_processed_once(monkeypatch: Any) -> None:
    calls = 0

    class _SpyConsumer:
        def __init__(self, _service: Any) -> None: ...

        async def handle(self, _name: str, _payload: dict[str, Any]) -> None:
            nonlocal calls
            calls += 1

    # Patch only the terminal chokepoint; the real RedisDedupe (fakeredis) does the deduping.
    monkeypatch.setattr(busrun, "NotificationEventConsumer", _SpyConsumer)

    handle = busrun.build_handler(_fake_app())
    payload = {"pl_id": "PLK1", "amount": "10", "currency": "KES"}
    await handle("paylink.verified", payload)
    await handle("paylink.verified", payload)  # at-least-once redelivery of the same event

    assert calls == 1  # the second delivery short-circuited on the Redis marker


async def test_distinct_events_each_processed(monkeypatch: Any) -> None:
    calls = 0

    class _SpyConsumer:
        def __init__(self, _service: Any) -> None: ...

        async def handle(self, _name: str, _payload: dict[str, Any]) -> None:
            nonlocal calls
            calls += 1

    monkeypatch.setattr(busrun, "NotificationEventConsumer", _SpyConsumer)

    handle = busrun.build_handler(_fake_app())
    await handle("paylink.verified", {"pl_id": "PLK1"})
    await handle("paylink.verified", {"pl_id": "PLK2"})  # different payload → not a duplicate

    assert calls == 2
