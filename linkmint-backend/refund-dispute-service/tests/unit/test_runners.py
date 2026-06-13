"""Background-runner glue: the bus consumer, the sweeper, and the outbox relay."""

from __future__ import annotations

import asyncio
import uuid
from datetime import UTC, datetime, timedelta
from types import SimpleNamespace
from typing import Any

import fakeredis.aioredis
import pytest

import app.busconsumer.run as bus_run
import app.sweeper.run as sweep_run
from app.busconsumer.run import topics_for
from app.clawback.coordinator import EventClawbackCoordinator
from app.events.relay import OutboxRelay
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from app.reversal.instruction import InstructionOnlyReversal
from app.reversal.port import RailReversalRegistry
from tests._support import (
    FakePaylinksClient,
    FakePaymentsClient,
    FakeRefundRepository,
    make_settings,
)


class _DummySession:
    """A session whose execute() satisfies DbDedupe (rowcount=1 → 'newly inserted')."""

    async def execute(self, *_a: Any, **_k: Any) -> SimpleNamespace:
        return SimpleNamespace(rowcount=1)

    async def commit(self) -> None: ...

    async def __aenter__(self) -> _DummySession:
        return self

    async def __aexit__(self, *_a: Any) -> bool:
        return False


def _app(repo: FakeRefundRepository, *, settings: Any = None) -> SimpleNamespace:
    state = SimpleNamespace(
        settings=settings or make_settings(),
        publisher=NoopPublisher(),
        payments_client=FakePaymentsClient(),
        paylinks_client=FakePaylinksClient(),
        reversal_registry=RailReversalRegistry(InstructionOnlyReversal()),
        clawback=EventClawbackCoordinator(),
        ledger_poster=NoopLedgerPoster(),
        redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
        sessionmaker=lambda: _DummySession(),
    )
    return SimpleNamespace(state=state)


# ── topic selection ──
def test_topics_is_chain() -> None:
    assert topics_for(make_settings()) == ["chain"]


# ── bus consumer ──
async def test_bus_handler_projects(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    monkeypatch.setattr(bus_run, "RefundRepository", lambda session: repo)
    app = _app(repo)
    handle = bus_run.build_handler(app)
    await handle(
        "chain.paylink.verified",
        {"entity_id": "0xpl", "tx_hash": "0xabc", "data": {"amount": 1000}},
    )
    vp = await repo.get_verified_paylink("0xpl")
    assert vp is not None and int(vp.amount_minor) == 1000


async def test_bus_handler_ignores_other(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    monkeypatch.setattr(bus_run, "RefundRepository", lambda session: repo)
    handle = bus_run.build_handler(_app(repo))
    await handle("merchant.onboarded", {"merchant_id": "m"})
    assert repo.verified == {}


async def test_bus_handler_redis_dedupes(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    calls = {"n": 0}

    async def counting_upsert(**kwargs: Any) -> None:
        calls["n"] += 1

    repo.upsert_verified_paylink = counting_upsert  # type: ignore[method-assign]
    monkeypatch.setattr(bus_run, "RefundRepository", lambda session: repo)
    handle = bus_run.build_handler(_app(repo))
    payload = {"entity_id": "0xpl", "tx_hash": "0xabc", "data": {"amount": 1}}
    await handle("chain.paylink.verified", payload)
    await handle("chain.paylink.verified", payload)  # same tx_hash → redis short-circuits
    assert calls["n"] == 1


async def test_bus_run_constructs_consumer(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    monkeypatch.setattr(bus_run, "RefundRepository", lambda session: repo)
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
    assert captured["topics"] == ["chain"]
    assert captured["ran"] is True


# ── sweeper ──
async def test_sweeper_run_once(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    monkeypatch.setattr(sweep_run, "RefundRepository", lambda session: repo)
    # an OPEN dispute past its window + a stale PROCESSING refund
    settings = make_settings(reversal_simulate=True, simulate_complete_after_seconds=1)
    app = _app(repo, settings=settings)

    from app.db.models import DisputeRow, RefundRow

    did = uuid.uuid4()
    repo.disputes[did] = DisputeRow(
        dispute_id=did,
        provider="stub",
        provider_dispute_id="x",
        payment_id="pay",
        paylink_id=None,
        rail="card",
        merchant_id=None,
        org_id=None,
        amount_minor=None,
        currency=None,
        reason_code=None,
        state="OPEN",
        evidence_due_at=datetime.now(UTC) - timedelta(hours=1),
        clawback_requested=False,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC),
    )
    rid = uuid.uuid4()
    repo.refunds[rid] = RefundRow(
        refund_id=rid,
        payment_id="pay",
        paylink_id="0xpl",
        rail="mpesa",
        requested_by="u",
        amount_minor=100,
        currency="KES",
        state="PROCESSING",
        is_partial=False,
        created_at=datetime.now(UTC),
        updated_at=datetime.now(UTC) - timedelta(hours=1),
    )
    expired, completed = await sweep_run._run_once(app)
    assert expired == 1
    assert completed == 1
    assert repo.disputes[did].state == "EXPIRED"
    assert repo.refunds[rid].state == "COMPLETED"


async def test_sweeper_loop_cancels(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakeRefundRepository()
    monkeypatch.setattr(sweep_run, "RefundRepository", lambda session: repo)

    async def fake_sleep(_s: float) -> None:
        raise asyncio.CancelledError()

    monkeypatch.setattr(sweep_run.asyncio, "sleep", fake_sleep)
    with pytest.raises(asyncio.CancelledError):
        await sweep_run.run(_app(repo))


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
        lambda: session, pub, schema="refund", table="refund_events", key_column="entity_id"
    )


async def test_relay_drain_once_publishes_and_marks() -> None:
    rows = [{"id": 1, "kind": "refund.completed", "key": "r-1", "payload": {"x": 1}}]
    session = _RelaySession(rows)
    pub = _FakePub()
    n = await _relay(session, pub)._drain_once()
    assert n == 1
    assert pub.published[0] == ("refund.completed", "r-1", {"x": 1})
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
