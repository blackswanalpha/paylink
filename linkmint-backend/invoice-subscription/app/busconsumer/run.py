"""work15 bus consumer — feeds ``chain.paylink.verified`` to InvoiceEventConsumer.

Runs as a lifespan background task when ``INVOICE_EVENT_CONSUMER_ENABLED=true``. Each event is
processed over a fresh DB session (mirroring the HTTP path's ``get_services``). Delivery is
at-least-once → a cheap best-effort RedisDedupe short-circuits common redeliveries, and the domain
transition is itself idempotent (only OPEN/OVERDUE → PAID).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI
from linkmint_idempotency import RedisDedupe, fingerprint

from app.config import Settings
from app.db.repositories import InvoiceRepository
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import InvoiceEventConsumer
from app.logging import get_logger

log = get_logger("invoice.busconsumer")

# Consumes chain.paylink.verified → the "chain" topic (the handler no-ops on anything else).
TOPICS = ["chain"]


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    settings: Settings = app.state.settings
    dedupe = RedisDedupe(app.state.redis, settings.service_name, settings.idempotency_ttl_seconds)

    async def handle(name: str, payload: dict[str, Any]) -> None:
        async def process() -> None:
            async with app.state.sessionmaker() as session:
                deps = ServiceDeps(
                    repo=InvoiceRepository(session),
                    commit=session.commit,
                    settings=settings,
                    publisher=app.state.publisher,
                    paylink=app.state.paylink_client,
                )
                await InvoiceEventConsumer(build_services(deps).invoices).handle(name, payload)

        await dedupe.run_once(name, fingerprint(payload), process)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    consumer = KafkaConsumer(settings.kafka_broker_list, TOPICS, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=TOPICS, group=settings.service_name)
    await consumer.run(build_handler(app))
