"""Async Kafka publisher (aiokafka), byte-compatible with eventbus-go's Publisher."""

from __future__ import annotations

from typing import Any

from aiokafka import AIOKafkaProducer
from opentelemetry import trace
from opentelemetry.propagate import inject
from opentelemetry.trace import SpanKind

from .envelope import Envelope
from .topics import topic_for


class KafkaPublisher:
    """Wraps a payload in a canonical Envelope and produces it synchronously — ``send_and_wait``
    blocks for the broker ack, giving at-least-once semantics."""

    def __init__(self, brokers: list[str], source: str, client_id: str = "linkmint") -> None:
        self._producer = AIOKafkaProducer(
            bootstrap_servers=brokers,
            client_id=client_id,
            acks="all",
            enable_idempotence=True,
        )
        self._source = source
        self._started = False

    async def start(self) -> None:
        await self._producer.start()
        self._started = True

    async def stop(self) -> None:
        if self._started:
            await self._producer.stop()
            self._started = False

    async def publish(
        self,
        name: str,
        key: str,
        payload: dict[str, Any] | None,
        correlation_id: str = "",
    ) -> None:
        env = Envelope.new(
            name=name,
            key=key,
            correlation_id=correlation_id,
            source=self._source,
            payload=payload,
        )
        # Start a producer span and ride the W3C trace context in Kafka record headers (no-op when
        # telemetry is off), so a consumer in any language continues the same trace. The Envelope
        # wire format is untouched — trace context lives only in the transport headers.
        with trace.get_tracer("eventbus").start_as_current_span(
            f"publish {name}", kind=SpanKind.PRODUCER
        ):
            carrier: dict[str, str] = {}
            inject(carrier)
            headers = [(k, v.encode("utf-8")) for k, v in carrier.items()] or None
            await self._producer.send_and_wait(
                topic_for(name),
                value=env.to_canonical_bytes(),
                key=key.encode("utf-8") if key else None,
                headers=headers,
            )
