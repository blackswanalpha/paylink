"""Domain-event publisher seam.

Events are referenced by their **logical name** (catalog.md). The durable record of every event is
the ``pricing.pricing_events`` outbox table, written in-transaction by the service; the work15 relay
drains it onto Kafka. The bus derives the topic from the name's first dot-segment
(``topic_for``), so these route to the ``pricing`` / ``fx`` / ``invoice`` topics respectively
(created by ``redpanda-init``) — see the catalog.md footnote. The inline :class:`Publisher` is an
in-process echo (log/noop) used until/unless the relay runs.

INVARIANT: payloads carry ids/amount metadata only — never secrets or PII (catalog.md).
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by fee-pricing-service (catalog.md line 74).
PRICING_FEE_QUOTE_ISSUED = "pricing.fee_quote.issued"  # → topic "pricing"
FX_RATE_UPDATED = "fx.rate.updated"  # → topic "fx"
INVOICE_PLATFORM_FEE_ISSUED = "invoice.platform_fee.issued"  # → topic "invoice"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
