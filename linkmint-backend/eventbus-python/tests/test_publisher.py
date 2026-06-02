from __future__ import annotations

from typing import Any

import pytest

from linkmint_eventbus import publisher as pub_mod
from linkmint_eventbus.envelope import Envelope


class FakeProducer:
    def __init__(self, **kwargs: Any) -> None:
        self.kwargs = kwargs
        self.started = False
        self.sent: list[tuple[str, bytes, bytes | None]] = []

    async def start(self) -> None:
        self.started = True

    async def stop(self) -> None:
        self.started = False

    async def send_and_wait(
        self, topic: str, value: bytes, key: bytes | None = None, headers: Any = None
    ) -> None:
        self.sent.append((topic, value, key))
        self.last_headers = headers


@pytest.fixture
def fake_producer(monkeypatch: pytest.MonkeyPatch) -> dict[str, FakeProducer]:
    created: dict[str, FakeProducer] = {}

    def factory(**kwargs: Any) -> FakeProducer:
        p = FakeProducer(**kwargs)
        created["p"] = p
        return p

    monkeypatch.setattr(pub_mod, "AIOKafkaProducer", factory)
    return created


async def test_publish_builds_canonical_envelope(fake_producer: dict[str, FakeProducer]) -> None:
    p = pub_mod.KafkaPublisher(["b:9092"], "paylink-service")
    await p.start()
    await p.publish(
        "paylink.verified", "PLK_1", {"pl_id": "PLK_1", "amount": "1000"}, correlation_id="trace-1"
    )
    fp = fake_producer["p"]
    assert fp.started
    assert fp.kwargs["acks"] == "all"
    topic, value, key = fp.sent[0]
    assert topic == "paylink"
    assert key == b"PLK_1"
    env = Envelope.from_bytes(value)
    assert env.name == "paylink.verified"
    assert env.source == "paylink-service"
    assert env.correlation_id == "trace-1"
    assert env.payload == {"amount": "1000", "pl_id": "PLK_1"}
    # canonical bytes: payload keys are sorted (amount before pl_id).
    assert value.index(b"amount") < value.index(b"pl_id")
    await p.stop()
    assert not fp.started


async def test_publish_without_key_sends_none(fake_producer: dict[str, FakeProducer]) -> None:
    p = pub_mod.KafkaPublisher(["b"], "payment-orchestrator")
    await p.publish("payment.failed", "", {"reason": "x"})
    _topic, _value, key = fake_producer["p"].sent[0]
    assert key is None
