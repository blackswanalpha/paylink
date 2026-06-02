from __future__ import annotations

import asyncio
from collections import namedtuple
from typing import Any

import pytest

from linkmint_eventbus import consumer as con_mod
from linkmint_eventbus.envelope import Envelope

Msg = namedtuple("Msg", "topic partition offset value")


class FakeConsumer:
    def __init__(self, *args: Any, **kwargs: Any) -> None:
        self.started = False
        self.stopped = False
        self.committed: list[dict[Any, int]] = []

    async def start(self) -> None:
        self.started = True

    async def stop(self) -> None:
        self.stopped = True

    async def commit(self, offsets: dict[Any, int]) -> None:
        self.committed.append(offsets)


def _bytes(name: str, payload: dict[str, Any]) -> bytes:
    return Envelope.new(name, "K", "", "src", payload).to_canonical_bytes()


@pytest.fixture
def consumer(monkeypatch: pytest.MonkeyPatch) -> con_mod.KafkaConsumer:
    monkeypatch.setattr(con_mod, "AIOKafkaConsumer", FakeConsumer)
    return con_mod.KafkaConsumer(["b"], ["paylink"], "grp")


async def test_all_handled_commits_last_offset_plus_one(consumer: con_mod.KafkaConsumer) -> None:
    seen: list[str] = []

    async def handle(name: str, payload: dict[str, Any]) -> None:
        seen.append(name)

    msgs = [
        Msg("paylink", 0, 0, _bytes("paylink.verified", {"a": 1})),
        Msg("paylink", 0, 1, _bytes("paylink.created", {"b": 2})),
    ]
    await consumer._process_partition("tp", msgs, handle)
    assert seen == ["paylink.verified", "paylink.created"]
    assert consumer._consumer.committed == [{"tp": 2}]  # type: ignore[attr-defined]


async def test_decode_failure_skips_and_commits(consumer: con_mod.KafkaConsumer) -> None:
    async def handle(name: str, payload: dict[str, Any]) -> None:
        raise AssertionError("handler must not be called for undecodable bytes")

    await consumer._process_partition("tp", [Msg("paylink", 0, 5, b"not-json")], handle)
    assert consumer._consumer.committed == [{"tp": 6}]  # type: ignore[attr-defined]


async def test_handle_failure_stops_and_commits_clean_prefix(
    consumer: con_mod.KafkaConsumer,
) -> None:
    calls: list[str] = []

    async def handle(name: str, payload: dict[str, Any]) -> None:
        calls.append(name)
        if name == "paylink.created":
            raise RuntimeError("boom")

    msgs = [
        Msg("paylink", 0, 0, _bytes("paylink.verified", {"a": 1})),
        Msg("paylink", 0, 1, _bytes("paylink.created", {"b": 2})),
        Msg("paylink", 0, 2, _bytes("paylink.cancelled", {"c": 3})),
    ]
    await consumer._process_partition("tp", msgs, handle)
    assert calls == ["paylink.verified", "paylink.created"]  # stops at the failure
    assert consumer._consumer.committed == [{"tp": 1}]  # type: ignore[attr-defined] # only offset 0 committed


class LoopConsumer(FakeConsumer):
    def __init__(self, batches: list[dict[Any, list[Msg]]]) -> None:
        super().__init__()
        self._batches = list(batches)

    async def getmany(self, timeout_ms: int, max_records: int) -> dict[Any, list[Msg]]:
        if self._batches:
            return self._batches.pop(0)
        raise asyncio.CancelledError


async def test_run_dispatches_batch_then_stops(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(con_mod, "AIOKafkaConsumer", FakeConsumer)
    c = con_mod.KafkaConsumer(["b"], ["paylink"], "grp")
    fake = LoopConsumer([{"tp": [Msg("paylink", 0, 0, _bytes("paylink.verified", {"a": 1}))]}])
    c._consumer = fake  # type: ignore[assignment]

    seen: list[str] = []

    async def handle(name: str, payload: dict[str, Any]) -> None:
        seen.append(name)

    with pytest.raises(asyncio.CancelledError):
        await c.run(handle)
    assert seen == ["paylink.verified"]
    assert fake.started and fake.stopped
    assert fake.committed == [{"tp": 1}]
