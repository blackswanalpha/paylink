"""work15 bus consumer — projects ``chain.paylink.verified`` into ``verified_paylinks``.

Runs as a lifespan background task when ``EVENT_CONSUMER_ENABLED=true``. Each event is processed
over
a fresh DB session. Delivery is at-least-once → a cheap RedisDedupe short-circuits common
redeliveries AND the durable DbDedupe row is written on the SAME transaction as the upsert, so the
projection (the original-amount source used for money decisions) applies exactly-once in effect.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI
from linkmint_idempotency import DbDedupe, RedisDedupe, fingerprint

from app.config import Settings
from app.db.repositories import RefundRepository
from app.events.consumer import CHAIN_PAYLINK_VERIFIED, ChainEventConsumer
from app.logging import get_logger

log = get_logger("refund.busconsumer")

_DEDUPE_SCOPE = CHAIN_PAYLINK_VERIFIED


def topics_for(_settings: Settings) -> list[str]:
    # chain-event-mirror republishes lVM events onto the `chain` topic (catalog.md).
    return ["chain"]


def _dedupe_key(name: str, payload: dict[str, Any]) -> str:
    """A stable per-event key: prefer the chain tx hash, else the entity id, else a payload
    digest."""
    return (
        str(payload.get("tx_hash")) or str(payload.get("entity_id") or "") or fingerprint(payload)
    )


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    settings: Settings = app.state.settings
    redis_dedupe = RedisDedupe(
        app.state.redis, settings.service_name, settings.idempotency_ttl_seconds
    )
    db_dedupe = DbDedupe()

    async def handle(name: str, payload: dict[str, Any]) -> None:
        if name != CHAIN_PAYLINK_VERIFIED:
            log.debug("event_ignored", event_name=name)
            return
        key = _dedupe_key(name, payload)

        async def process() -> None:
            async with app.state.sessionmaker() as session:
                repo = RefundRepository(session)

                async def apply() -> None:
                    consumer = ChainEventConsumer(_RepoSink(repo))
                    await consumer.handle(name, payload)

                # The dedupe row + the upsert commit together (exactly-once effect).
                ran, _ = await db_dedupe.run_once(session, _DEDUPE_SCOPE, key, apply)
                await session.commit()
                if ran:
                    log.info("chain_event_projected", key=key)

        await redis_dedupe.run_once(_DEDUPE_SCOPE, key, process)

    return handle


class _RepoSink:
    """Adapts RefundRepository to the ChainEventConsumer's VerifiedPaylinkSink protocol."""

    def __init__(self, repo: RefundRepository) -> None:
        self._repo = repo

    async def project_verified_paylink(self, **kwargs: Any) -> None:
        await self._repo.upsert_verified_paylink(**kwargs)


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    topics = topics_for(settings)
    consumer = KafkaConsumer(settings.kafka_broker_list, topics, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=topics, group=settings.service_name)
    await consumer.run(build_handler(app))
