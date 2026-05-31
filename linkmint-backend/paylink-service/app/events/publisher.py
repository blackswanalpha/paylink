"""Domain-event publisher seam.

Events are referenced by their **logical name** (the backendfeatures.md taxonomy). The concrete
Kafka/SQS transport (ADR-004) is delivered by **work15**; until then a publisher just logs or
no-ops. The durable record of every event is the ``paylink.paylink_events`` table, written
in-transaction by the service (so work15 can drain it as an outbox).
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by paylink-service.
PAYLINK_REQUESTED = "paylink.requested"
PAYLINK_CREATED = "paylink.created"
PAYLINK_CANCELLED = "paylink.cancelled"
PAYLINK_EXPIRED = "paylink.expired"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
