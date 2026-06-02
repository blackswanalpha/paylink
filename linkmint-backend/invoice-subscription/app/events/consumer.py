"""Inbound event consumer — settlement truth from the chain.

invoice-subscription consumes ``chain.paylink.verified`` (republished by chain-event-mirror onto the
``chain`` topic). The event's ``entity_id`` is the verified PayLink id; we mark the invoice it backs
as PAID. Delivery is at-least-once, so the handler is idempotent (``mark_paid_by_plid`` only
transitions OPEN/OVERDUE → PAID). Unknown events / missing fields → log + no-op.
"""

from __future__ import annotations

from typing import Any, Protocol

from app.logging import get_logger

log = get_logger("invoice.consumer")

CHAIN_PAYLINK_VERIFIED = "chain.paylink.verified"


class InvoiceMarker(Protocol):
    async def mark_paid_by_plid(self, pl_id: str) -> bool: ...


class InvoiceEventConsumer:
    def __init__(self, invoices: InvoiceMarker) -> None:
        self._invoices = invoices

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        if name != CHAIN_PAYLINK_VERIFIED:
            log.debug("event_ignored", event_name=name)
            return
        # chain-event-mirror payload carries the PayLink id as ``entity_id``; accept ``pl_id`` too.
        pl_id = payload.get("entity_id") or payload.get("pl_id")
        if not pl_id:
            log.warning("paid_event_missing_plid", event_name=name)
            return
        await self._invoices.mark_paid_by_plid(str(pl_id))
