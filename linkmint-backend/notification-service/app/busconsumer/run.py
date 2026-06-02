"""work15 bus consumer — feeds bus events to the existing NotificationEventConsumer chokepoint.

Runs as a lifespan background task when ``NOTIFY_EVENT_CONSUMER_ENABLED=true``. Each event is
processed over a fresh DB session (mirroring the HTTP intake's ``get_consumer``), so the bus and
the HTTP paths share one ``handle`` chokepoint. Delivery is at-least-once, so the handler is
idempotent (per-event DB dedupe — duplicates collapse on the ``deliveries_dedupe_uidx`` index).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI
from linkmint_idempotency import RedisDedupe, fingerprint

from app.config import Settings
from app.db.repository import NotifyRepository
from app.domain.service import NotificationService
from app.events.consumer import NotificationEventConsumer
from app.logging import get_logger
from app.templating.registry import TemplateRegistry

log = get_logger("notify.busconsumer")

# Consumes paylink.verified + payment.failed → the "paylink" and "payment" topics (the handler
# dispatches by logical name and no-ops on anything else).
TOPICS = ["paylink", "payment"]


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    """An async handle(name, payload) bound to app state, building the service per message."""
    settings: Settings = app.state.settings
    # work17 — cheap best-effort short-circuit in front of the durable per-delivery DB dedupe
    # (deliveries_dedupe_uidx): skip re-rendering/re-enqueuing when the same event redelivers. The
    # DB UNIQUE stays the exactly-once arbiter; this just avoids repeating work on the common path.
    dedupe = RedisDedupe(app.state.redis, settings.service_name, settings.idempotency_ttl_seconds)

    async def handle(name: str, payload: dict[str, Any]) -> None:
        async def process() -> None:
            async with app.state.sessionmaker() as session:
                repo = NotifyRepository(session)
                service = NotificationService(
                    repo=repo,
                    registry=TemplateRegistry(repo),
                    resolver=app.state.recipient_resolver,
                    enqueue=app.state.enqueue,
                    commit=session.commit,
                )
                await NotificationEventConsumer(service).handle(name, payload)

        await dedupe.run_once(name, fingerprint(payload), process)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    consumer = KafkaConsumer(settings.kafka_broker_list, TOPICS, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=TOPICS, group=settings.service_name)
    await consumer.run(build_handler(app))
