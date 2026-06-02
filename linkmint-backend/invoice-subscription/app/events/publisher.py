"""Domain-event publisher seam.

Events are referenced by their **logical name** (backendfeatures.md §2.19). The durable record of
every event is the ``invoice.invoice_events`` outbox table, written in-transaction by the service;
the work15 relay drains it onto Kafka (topic ``invoice``). The inline :class:`Publisher` is an
in-process echo (log/noop) used until/unless the relay runs.

INVARIANT: payloads carry ids/amount metadata only — never secrets or PII.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by invoice-subscription (backendfeatures.md §2.19).
INVOICE_CREATED = "invoice.created"
INVOICE_FINALIZED = "invoice.finalized"
INVOICE_PAID = "invoice.paid"
INVOICE_OVERDUE = "invoice.overdue"
INVOICE_VOIDED = "invoice.voided"  # beyond the §2.19 four — a useful lifecycle signal


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
