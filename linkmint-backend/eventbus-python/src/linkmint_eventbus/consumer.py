"""Async Kafka consumer-group member (aiokafka) with commit-after-handle (at-least-once)."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from aiokafka import AIOKafkaConsumer
from opentelemetry import context, trace
from opentelemetry.context import Context
from opentelemetry.propagate import extract
from opentelemetry.trace import SpanKind

from .envelope import Envelope
from .logging import get_logger
from .metrics import BUS_MESSAGES_CONSUMED

# A handler receives the logical event name and the decoded payload, mirroring the services'
# existing typed ``handle(name, payload)`` chokepoints.
HandleFunc = Callable[[str, dict[str, Any]], Awaitable[Any]]


class KafkaConsumer:
    """Decodes envelopes and dispatches them to a handler, committing offsets only after a clean
    handle. Within a partition it commits the clean prefix and stops at the first handle error (the
    failed message redelivers); an undecodable message is logged and skipped+committed (poison
    safe). Duplicates are possible (retry/rebalance), so handlers MUST be idempotent.
    """

    def __init__(
        self,
        brokers: list[str],
        topics: list[str],
        group_id: str,
        client_id: str = "linkmint",
        log: Any | None = None,
    ) -> None:
        self._consumer = AIOKafkaConsumer(
            *topics,
            bootstrap_servers=brokers,
            group_id=group_id,
            client_id=client_id,
            enable_auto_commit=False,
            # A new group reads from the earliest record so a newly-added consumer never silently
            # skips events produced before it joined; once it commits it resumes from there.
            auto_offset_reset="earliest",
        )
        self._log = log or get_logger("eventbus.consumer")

    async def run(self, handle: HandleFunc) -> None:
        """Poll and dispatch until the task is cancelled; then stop the consumer cleanly."""
        await self._consumer.start()
        try:
            while True:
                batches = await self._consumer.getmany(timeout_ms=1000, max_records=256)
                for tp, msgs in batches.items():
                    await self._process_partition(tp, msgs, handle)
        finally:
            await self._consumer.stop()

    async def _process_partition(self, tp: Any, msgs: list[Any], handle: HandleFunc) -> None:
        commit_offset: int | None = None
        for msg in msgs:
            try:
                env = Envelope.from_bytes(msg.value)
            except Exception:  # noqa: BLE001 — undecodable bytes can never succeed
                self._log.warning("eventbus_decode_failed", topic=msg.topic, offset=msg.offset)
                commit_offset = msg.offset + 1  # poison-safe: skip + commit
                continue
            # Continue the producer's trace under a consume span; the handler runs within it.
            span = trace.get_tracer("eventbus").start_span(
                f"consume {env.name}",
                context=_extract_trace(getattr(msg, "headers", None)),
                kind=SpanKind.CONSUMER,
            )
            token = context.attach(trace.set_span_in_context(span))
            failed = False
            try:
                await handle(env.name, env.payload)
            except Exception as exc:  # noqa: BLE001 — any handler error → do not commit → redeliver
                failed = True
                BUS_MESSAGES_CONSUMED.labels(env.name, "error").inc()
                self._log.warning(
                    "eventbus_handle_failed",
                    name=env.name,
                    key=env.key,
                    offset=msg.offset,
                    error=str(exc),
                )
            else:
                BUS_MESSAGES_CONSUMED.labels(env.name, "ok").inc()
            finally:
                span.end()
                context.detach(token)
            if failed:
                break  # stop this partition; the failed message + the rest redeliver
            commit_offset = msg.offset + 1
        if commit_offset is not None:
            await self._consumer.commit({tp: commit_offset})


def _extract_trace(headers: Any) -> Context:
    """Rebuild the W3C trace context from a Kafka message's headers (list of (str, bytes) pairs)."""
    carrier: dict[str, str] = {}
    for item in headers or []:
        k, v = item
        carrier[k] = v.decode("utf-8") if isinstance(v, (bytes, bytearray)) else str(v)
    return extract(carrier)
