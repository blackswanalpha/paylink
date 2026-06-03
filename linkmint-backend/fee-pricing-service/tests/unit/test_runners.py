"""Background-runner glue: the bus consumer, topic selection, and the outbox relay."""

from __future__ import annotations

import asyncio
import uuid
from types import SimpleNamespace
from typing import Any

import fakeredis.aioredis
import pytest

import app.busconsumer.run as bus_run
from app.busconsumer.run import topics_for
from app.events.relay import OutboxRelay
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from tests._support import FakeFxProvider, FakePricingRepository, make_settings


class _DummySession:
    async def commit(self) -> None: ...

    async def __aenter__(self) -> _DummySession:
        return self

    async def __aexit__(self, *_a: Any) -> bool:
        return False


def _app(repo: FakePricingRepository, *, settings: Any = None) -> SimpleNamespace:
    state = SimpleNamespace(
        settings=settings or make_settings(),
        publisher=NoopPublisher(),
        fx_provider=FakeFxProvider(),
        redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
        ledger_poster=NoopLedgerPoster(),
        sessionmaker=lambda: _DummySession(),
    )
    return SimpleNamespace(state=state)


# ── topic selection ──
def test_topics_default_is_merchant() -> None:
    assert topics_for(make_settings()) == ["merchant"]


def test_topics_adds_chain_when_accrual_seam_on() -> None:
    assert topics_for(make_settings(accrual_from_events=True)) == ["merchant", "chain"]


# ── bus consumer ──
async def test_bus_handler_applies_merchant_event(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakePricingRepository()
    monkeypatch.setattr(bus_run, "PricingRepository", lambda session: repo)
    mid, org = uuid.uuid4(), uuid.uuid4()
    app = _app(repo)
    handle = bus_run.build_handler(app)
    await handle("merchant.onboarded", {"merchant_id": str(mid), "org_id": str(org)})
    assert repo.merchant_pricing[mid].tier == "standard"


async def test_bus_run_constructs_consumer(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakePricingRepository()
    monkeypatch.setattr(bus_run, "PricingRepository", lambda session: repo)
    captured: dict[str, Any] = {}

    class FakeConsumer:
        def __init__(self, brokers: Any, topics: Any, group_id: Any) -> None:
            captured["topics"] = topics
            captured["group"] = group_id

        async def run(self, handler: Any) -> None:
            captured["ran"] = True

    import linkmint_eventbus

    monkeypatch.setattr(linkmint_eventbus, "KafkaConsumer", FakeConsumer)
    await bus_run.run(_app(repo, settings=make_settings(kafka_brokers="b:9092")))
    assert captured["topics"] == ["merchant"]
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
        lambda: session, pub, schema="pricing", table="pricing_events", key_column="entity_id"
    )


async def test_relay_drain_once_publishes_and_marks() -> None:
    rows = [{"id": 1, "kind": "fx.rate.updated", "key": "USD:KES", "payload": {"rate": "129.5"}}]
    session = _RelaySession(rows)
    pub = _FakePub()
    n = await _relay(session, pub)._drain_once()
    assert n == 1
    assert pub.published[0] == ("fx.rate.updated", "USD:KES", {"rate": "129.5"})
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
