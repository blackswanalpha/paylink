"""work15 bus consumer — feeds paylink.requested / payment.initiated to ComplianceEventConsumer.

Runs as a lifespan background task when ``COMPLIANCE_EVENT_CONSUMER_ENABLED=true``. Each event is
processed over a fresh DB session (mirroring the HTTP path's ``get_services``); the consumer records
the value action into the activity ledger for velocity/AML checks. Delivery is at-least-once → the
handler is idempotent (the ledger insert is keyed off the event so duplicates collapse).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import ComplianceRepository
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import ComplianceEventConsumer
from app.logging import get_logger

log = get_logger("compliance.busconsumer")

# Consumes paylink.requested (topic "paylink") + payment.initiated (topic "payment").
TOPICS = ["paylink", "payment"]


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    async def handle(name: str, payload: dict[str, Any]) -> None:
        async with app.state.sessionmaker() as session:
            deps = ServiceDeps(
                repo=ComplianceRepository(session),
                commit=session.commit,
                settings=app.state.settings,
                publisher=app.state.publisher,
                cipher=app.state.provider_cipher,
            )
            await ComplianceEventConsumer(build_services(deps).risk).handle(name, payload)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    consumer = KafkaConsumer(settings.kafka_broker_list, TOPICS, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=TOPICS, group=settings.service_name)
    await consumer.run(build_handler(app))
