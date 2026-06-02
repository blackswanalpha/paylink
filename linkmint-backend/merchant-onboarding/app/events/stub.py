"""Publisher implementations used until the real transport lands (work15)."""

from __future__ import annotations

from typing import Any

from app.config import Settings
from app.events.publisher import Publisher
from app.logging import get_logger

log = get_logger("merchant.events")


class LogPublisher(Publisher):
    async def publish(self, name: str, payload: dict[str, Any]) -> None:
        # `event_name` (not `event`) — structlog binds the positional message to `event`.
        log.info("domain_event", event_name=name, payload=payload)


class NoopPublisher(Publisher):
    async def publish(self, name: str, payload: dict[str, Any]) -> None:
        return None


def build_publisher(settings: Settings) -> Publisher:
    # In "kafka" mode the inline seam goes quiet — the outbox-drain relay (app/events/relay.py) is
    # the real Kafka producer. "log" keeps the in-process echo; "noop" silences it.
    if settings.event_publisher_mode in ("noop", "kafka"):
        return NoopPublisher()
    return LogPublisher()
