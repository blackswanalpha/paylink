"""work15 bus consumer — feeds compliance.kyc.* events to KycConsumer.handle.

Runs as a lifespan background task when ``IDENTITY_EVENT_CONSUMER_ENABLED=true``. Each event is
processed over a fresh DB session (mirroring the HTTP path's ``get_services``). Delivery is
at-least-once → the handler is idempotent (setting a KYC tier is idempotent).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import IdentityRepository
from app.domain.services import ServiceDeps, build_services
from app.events.consumer import KycConsumer
from app.logging import get_logger

log = get_logger("identity.busconsumer")

# Consumes compliance.kyc.passed/failed → the "compliance" topic.
TOPICS = ["compliance"]


def build_handler(app: FastAPI) -> Callable[[str, dict[str, Any]], Awaitable[None]]:
    async def handle(name: str, payload: dict[str, Any]) -> None:
        async with app.state.sessionmaker() as session:
            deps = ServiceDeps(
                repo=IdentityRepository(session),
                commit=session.commit,
                settings=app.state.settings,
                publisher=app.state.publisher,
                passwords=app.state.passwords,
                jwt=app.state.jwt_issuer,
                mfa_cipher=app.state.mfa_cipher,
                oauth=app.state.oauth_resolver,
                failed_login=app.state.failed_login,
            )
            await KycConsumer(build_services(deps).users).handle(name, payload)

    return handle


async def run(app: FastAPI) -> None:
    from linkmint_eventbus import KafkaConsumer

    settings: Settings = app.state.settings
    consumer = KafkaConsumer(settings.kafka_broker_list, TOPICS, group_id=settings.service_name)
    log.info("bus_consumer_started", topics=TOPICS, group=settings.service_name)
    await consumer.run(build_handler(app))
