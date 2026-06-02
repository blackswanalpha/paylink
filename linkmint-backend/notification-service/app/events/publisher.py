"""Domain-event publisher seam (forward-symmetry).

notification-service is a terminal consumer — in Phase 1 it produces no domain events. This seam
exists so a future ``notification.delivered`` / ``notification.exhausted`` event drops in the same
way other services publish (LogPublisher/NoopPublisher until work15's Kafka/SQS transport lands).
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

from app.config import Settings
from app.logging import get_logger

log = get_logger("notify.events")

# Logical event names notification-service may produce (Phase 2).
NOTIFICATION_DELIVERED = "notification.delivered"
NOTIFICATION_EXHAUSTED = "notification.exhausted"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...


class LogPublisher(Publisher):
    async def publish(self, name: str, payload: dict[str, Any]) -> None:
        log.info("domain_event", event_name=name, payload=payload)


class NoopPublisher(Publisher):
    async def publish(self, name: str, payload: dict[str, Any]) -> None:
        return None


def build_publisher(settings: Settings) -> Publisher:
    if settings.event_publisher_mode == "noop":
        return NoopPublisher()
    return LogPublisher()
