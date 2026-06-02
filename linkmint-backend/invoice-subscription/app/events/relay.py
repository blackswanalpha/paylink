"""Outbox-drain relay (work15) — the transactional-outbox producer.

The domain writes an event row into ``invoice.invoice_events`` in the same transaction as the
business change (the source of truth). This background task drains unpublished rows to the Kafka bus
via ``linkmint_eventbus`` and marks them published. Runs only when ``EVENT_PUBLISHER_MODE=kafka``.

At-least-once: a crash after a publish but before the row is marked republishes the row on restart
(consumers are idempotent — pairs with work17). Payloads carry ids/metadata only, never secrets.
"""

from __future__ import annotations

import asyncio

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from app.logging import get_logger

log = get_logger("invoice.relay")


class OutboxRelay:
    """Drains an outbox (id, kind, <key_column>, payload, published_at) to the bus."""

    def __init__(
        self,
        sessionmaker: async_sessionmaker[AsyncSession],
        publisher: object,  # linkmint_eventbus.KafkaPublisher (duck-typed to avoid a hard import)
        *,
        schema: str,
        table: str,
        key_column: str,
        poll_interval: float = 1.0,
        batch: int = 100,
    ) -> None:
        self._sm = sessionmaker
        self._pub = publisher
        self._poll = poll_interval
        self._batch = batch
        # FOR UPDATE SKIP LOCKED lets multiple relay instances coexist without double-publishing.
        self._select = text(
            f"SELECT id, kind, ({key_column})::text AS key, payload "
            f"FROM {schema}.{table} WHERE published_at IS NULL "
            f"ORDER BY id LIMIT :n FOR UPDATE SKIP LOCKED"
        )
        self._mark = text(f"UPDATE {schema}.{table} SET published_at = now() WHERE id = :id")

    async def run(self) -> None:
        await self._pub.start()  # type: ignore[attr-defined]
        try:
            while True:
                try:
                    published = await self._drain_once()
                except asyncio.CancelledError:
                    raise
                except Exception as exc:  # noqa: BLE001 — a transient blip must not kill the loop
                    log.warning("outbox_drain_error", error=str(exc))
                    published = 0
                if published == 0:
                    await asyncio.sleep(self._poll)
        finally:
            await self._pub.stop()  # type: ignore[attr-defined]

    async def _drain_once(self) -> int:
        async with self._sm() as session:
            rows = (await session.execute(self._select, {"n": self._batch})).mappings().all()
            for row in rows:
                await self._pub.publish(row["kind"], row["key"] or "", row["payload"])  # type: ignore[attr-defined]
                await session.execute(self._mark, {"id": row["id"]})
            await session.commit()
        if rows:
            log.info("outbox_drained", count=len(rows))
        return len(rows)
