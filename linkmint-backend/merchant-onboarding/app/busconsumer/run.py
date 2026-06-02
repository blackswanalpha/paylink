"""work15 bus consumer — feeds compliance.kyb.* events to MerchantEventConsumer.handle.

Runs as a lifespan background task when ``MERCHANT_EVENT_CONSUMER_ENABLED=true``. Each event is
processed over a fresh DB session (mirroring the HTTP path's ``get_services``). Delivery is
at-least-once → the handler is idempotent.

Note: the consumer also handles ``admin.override.*`` (topic "admin"), produced by admin-backoffice;
that producer/topic is a Phase-2 follow-up, so only "compliance" is subscribed here.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import MerchantRepository
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import MerchantEventConsumer
from app.logging import get_logger

log = get_logger("merchant.busconsumer")

# Consumes compliance.kyb.passed/failed → the "compliance" topic.
TOPICS = ["compliance"]


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    async def handle(name: str, payload: dict[str, Any]) -> None:
        async with app.state.sessionmaker() as session:
            deps = ServiceDeps(
                repo=MerchantRepository(session),
                commit=session.commit,
                settings=app.state.settings,
                publisher=app.state.publisher,
                bank_cipher=app.state.bank_cipher,
                object_store=app.state.object_store,
            )
            await MerchantEventConsumer(build_services(deps).merchants).handle(name, payload)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    consumer = KafkaConsumer(settings.kafka_broker_list, TOPICS, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=TOPICS, group=settings.service_name)
    await consumer.run(build_handler(app))
