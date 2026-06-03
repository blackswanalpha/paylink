"""work15 bus consumer — feeds ``merchant.*`` events to the MerchantPricingEventConsumer.

Runs as a lifespan background task when ``EVENT_CONSUMER_ENABLED=true``. Each event is processed
over a fresh DB session (mirroring the HTTP path's ``get_services``). Delivery is at-least-once → a
cheap best-effort RedisDedupe short-circuits common redeliveries, and the upsert is itself
idempotent. The optional ``chain`` topic is subscribed only when ``ACCRUAL_FROM_EVENTS=true`` (the
accrual seam; the fee-bearing producing event isn't fixed yet, so the handler just logs it).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI
from linkmint_idempotency import RedisDedupe, fingerprint

from app.config import Settings
from app.db.repositories import PricingRepository
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import MerchantPricingEventConsumer
from app.logging import get_logger

log = get_logger("pricing.busconsumer")


def topics_for(settings: Settings) -> list[str]:
    topics = ["merchant"]  # merchant-onboarding emits onto the `merchant` topic (catalog.md)
    if settings.accrual_from_events:
        topics.append("chain")
    return topics


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    settings: Settings = app.state.settings
    dedupe = RedisDedupe(app.state.redis, settings.service_name, settings.idempotency_ttl_seconds)

    async def handle(name: str, payload: dict[str, Any]) -> None:
        async def process() -> None:
            async with app.state.sessionmaker() as session:
                deps = ServiceDeps(
                    repo=PricingRepository(session),
                    commit=session.commit,
                    settings=settings,
                    publisher=app.state.publisher,
                    fx_provider=app.state.fx_provider,
                    redis=app.state.redis,
                    ledger=app.state.ledger_poster,
                )
                await MerchantPricingEventConsumer(build_services(deps).pricing).handle(
                    name, payload
                )

        await dedupe.run_once(name, fingerprint(payload), process)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    topics = topics_for(settings)
    consumer = KafkaConsumer(settings.kafka_broker_list, topics, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=topics, group=settings.service_name)
    await consumer.run(build_handler(app))
