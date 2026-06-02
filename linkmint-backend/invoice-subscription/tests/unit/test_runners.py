"""Background-runner glue: the overdue sweeper loop, the bus consumer, and the outbox relay.

These exercise the lifespan tasks without a real DB/Kafka by faking the session/repo and (for the
relay) the publisher + a duck-typed session.
"""

from __future__ import annotations

import asyncio
import uuid
from datetime import UTC, datetime, timedelta
from types import SimpleNamespace
from typing import Any

import fakeredis.aioredis
import pytest

import app.busconsumer.run as bus_run
import app.sweeper.run as sweeper_run
from app.events.relay import OutboxRelay
from app.events.stub import NoopPublisher
from tests._support import FakeInvoiceRepository, FakePaylink, make_settings


class _DummySession:
    async def commit(self) -> None: ...

    async def __aenter__(self) -> _DummySession:
        return self

    async def __aexit__(self, *_a: Any) -> bool:
        return False


def _app(
    repo: FakeInvoiceRepository, *, redis: Any = None, settings: Any = None
) -> SimpleNamespace:
    state = SimpleNamespace(
        settings=settings or make_settings(),
        publisher=NoopPublisher(),
        paylink_client=FakePaylink(),
        sessionmaker=lambda: _DummySession(),
        redis=redis,
    )
    return SimpleNamespace(state=state)


# ── sweeper ──
async def test_sweep_once_marks_overdue(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeInvoiceRepository()
    repo.seed(merchant_id=uuid.uuid4(), status="OPEN", due_at=datetime.now(UTC) - timedelta(days=1))
    monkeypatch.setattr(sweeper_run, "InvoiceRepository", lambda session: repo)
    assert await sweeper_run._sweep_once(_app(repo)) == 1


async def test_sweeper_run_swallows_error_then_cancels(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeInvoiceRepository()
    seen = {"n": 0}

    async def boom(_app: Any) -> int:
        seen["n"] += 1
        raise RuntimeError("transient")

    async def fake_sleep(_s: float) -> None:
        raise asyncio.CancelledError()

    monkeypatch.setattr(sweeper_run, "_sweep_once", boom)
    monkeypatch.setattr(sweeper_run.asyncio, "sleep", fake_sleep)
    with pytest.raises(asyncio.CancelledError):
        await sweeper_run.run(_app(repo))
    assert seen["n"] == 1


# ── bus consumer ──
async def test_bus_handler_marks_paid(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeInvoiceRepository()
    row = repo.seed(merchant_id=uuid.uuid4(), status="OPEN", pl_id="PLK_bus")
    monkeypatch.setattr(bus_run, "InvoiceRepository", lambda session: repo)
    app = _app(repo, redis=fakeredis.aioredis.FakeRedis(decode_responses=True))
    handle = bus_run.build_handler(app)
    await handle("chain.paylink.verified", {"entity_id": "PLK_bus"})
    assert row.status == "PAID"


async def test_bus_run_constructs_consumer(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeInvoiceRepository()
    monkeypatch.setattr(bus_run, "InvoiceRepository", lambda session: repo)
    captured: dict[str, Any] = {}

    class FakeConsumer:
        def __init__(self, brokers: Any, topics: Any, group_id: Any) -> None:
            captured["topics"] = topics
            captured["group"] = group_id

        async def run(self, handler: Any) -> None:
            captured["ran"] = True

    import linkmint_eventbus

    monkeypatch.setattr(linkmint_eventbus, "KafkaConsumer", FakeConsumer)
    app = _app(
        repo,
        redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
        settings=make_settings(kafka_brokers="b:9092"),
    )
    await bus_run.run(app)
    assert captured["topics"] == ["chain"]
    assert captured["ran"] is True


# ── outbox relay ──
class _FakePub:
    def __init__(self) -> None:
        self.published: list[tuple[str, str, Any]] = []
        self.started = False
        self.stopped = False

    async def start(self) -> None:
        self.started = True

    async def stop(self) -> None:
        self.stopped = True

    async def publish(self, name: str, key: str, payload: Any) -> None:
        self.published.append((name, key, payload))


class _Result:
    def __init__(self, rows: list[dict[str, Any]]) -> None:
        self._rows = rows

    def mappings(self) -> _Result:
        return self

    def all(self) -> list[dict[str, Any]]:
        return self._rows


class _RelaySession:
    def __init__(self, rows: list[dict[str, Any]]) -> None:
        self._rows = rows
        self.committed = False

    async def execute(self, stmt: Any, params: Any = None) -> _Result:
        return _Result(self._rows) if "SELECT" in str(stmt).upper() else _Result([])

    async def commit(self) -> None:
        self.committed = True

    async def __aenter__(self) -> _RelaySession:
        return self

    async def __aexit__(self, *_a: Any) -> bool:
        return False


def _relay(session: _RelaySession, pub: _FakePub) -> OutboxRelay:
    return OutboxRelay(
        lambda: session, pub, schema="invoice", table="invoice_events", key_column="invoice_id"
    )


async def test_relay_drain_once_publishes_and_marks() -> None:
    rows = [{"id": 1, "kind": "invoice.created", "key": "k1", "payload": {"x": 1}}]
    session = _RelaySession(rows)
    pub = _FakePub()
    n = await _relay(session, pub)._drain_once()
    assert n == 1
    assert pub.published[0] == ("invoice.created", "k1", {"x": 1})
    assert session.committed


async def test_relay_run_starts_and_stops(monkeypatch: pytest.MonkeyPatch) -> None:
    pub = _FakePub()
    relay = _relay(_RelaySession([]), pub)

    async def fake_sleep(_s: float) -> None:
        raise asyncio.CancelledError()

    import app.events.relay as relay_mod

    monkeypatch.setattr(relay_mod.asyncio, "sleep", fake_sleep)
    with pytest.raises(asyncio.CancelledError):
        await relay.run()
    assert pub.started and pub.stopped


async def test_relay_run_swallows_drain_error(monkeypatch: pytest.MonkeyPatch) -> None:
    pub = _FakePub()
    relay = _relay(_RelaySession([]), pub)

    async def boom() -> int:
        raise RuntimeError("blip")

    async def fake_sleep(_s: float) -> None:
        raise asyncio.CancelledError()

    import app.events.relay as relay_mod

    monkeypatch.setattr(relay, "_drain_once", boom)
    monkeypatch.setattr(relay_mod.asyncio, "sleep", fake_sleep)
    with pytest.raises(asyncio.CancelledError):
        await relay.run()
    assert pub.stopped
