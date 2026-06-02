"""Cross-service bus trace propagation: a publish injects W3C trace context into Kafka headers and a
consume continues the same trace — verified end-to-end without a broker, with a real SDK provider.
"""

from __future__ import annotations

from collections import namedtuple
from typing import Any

import pytest
from opentelemetry import trace
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.propagate import set_global_textmap
from opentelemetry.propagators.composite import CompositePropagator
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

from linkmint_eventbus import consumer as con_mod
from linkmint_eventbus import publisher as pub_mod
from linkmint_eventbus.envelope import Envelope

Msg = namedtuple("Msg", "topic partition offset value headers")


@pytest.fixture(scope="module", autouse=True)
def _otel() -> Any:
    set_global_textmap(
        CompositePropagator([TraceContextTextMapPropagator(), W3CBaggagePropagator()])
    )
    trace.set_tracer_provider(TracerProvider(sampler=ALWAYS_ON))
    yield


class FakeProducer:
    def __init__(self, **kwargs: Any) -> None:
        self.headers: Any = None

    async def start(self) -> None: ...

    async def stop(self) -> None: ...

    async def send_and_wait(
        self, topic: str, value: bytes, key: bytes | None = None, headers: Any = None
    ) -> None:
        self.headers = headers


class FakeConsumer:
    def __init__(self, *a: Any, **k: Any) -> None:
        self.committed: list[Any] = []

    async def start(self) -> None: ...

    async def stop(self) -> None: ...

    async def commit(self, offsets: Any) -> None:
        self.committed.append(offsets)


def _bytes(name: str, payload: dict[str, Any]) -> bytes:
    return Envelope.new(name, "K", "", "src", payload).to_canonical_bytes()


async def test_publish_injects_and_consumer_continues(monkeypatch: pytest.MonkeyPatch) -> None:
    fp = FakeProducer()
    monkeypatch.setattr(pub_mod, "AIOKafkaProducer", lambda **k: fp)
    p = pub_mod.KafkaPublisher(["b"], "svc")

    with trace.get_tracer("t").start_as_current_span("root") as span:
        root_trace = format(span.get_span_context().trace_id, "032x")
        await p.publish("paylink.verified", "K", {"a": 1})

    headers = fp.headers
    assert headers and any(k == "traceparent" for k, _ in headers)

    monkeypatch.setattr(con_mod, "AIOKafkaConsumer", FakeConsumer)
    c = con_mod.KafkaConsumer(["b"], ["paylink"], "grp")
    seen: dict[str, str] = {}

    async def handle(name: str, payload: dict[str, Any]) -> None:
        seen["trace"] = format(trace.get_current_span().get_span_context().trace_id, "032x")

    msg = Msg("paylink", 0, 0, _bytes("paylink.verified", {"a": 1}), headers)
    await c._process_partition("tp", [msg], handle)

    assert seen["trace"] == root_trace  # consumer continued the producer's trace
    assert c._consumer.committed == [{"tp": 1}]  # type: ignore[attr-defined]
