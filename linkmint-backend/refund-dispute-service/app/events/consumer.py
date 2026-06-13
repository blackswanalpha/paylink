"""Inbound event consumer — projects settlement truth from the chain.

refund-dispute-service consumes ``chain.paylink.verified`` (republished by chain-event-mirror onto
the ``chain`` topic). The event's ``entity_id`` is the verified PayLink id; we upsert it into
``verified_paylinks`` so the refund path has the authoritative original amount (A.3) when validating
full/partial refunds. Delivery is at-least-once, so the handler is idempotent (the upsert is, and
the
bus consumer wraps it in DbDedupe). Unknown events / missing fields → log + no-op.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any, Protocol

from app.logging import get_logger

log = get_logger("refund.consumer")

CHAIN_PAYLINK_VERIFIED = "chain.paylink.verified"


class VerifiedPaylinkSink(Protocol):
    async def project_verified_paylink(
        self,
        *,
        paylink_id: str,
        tx_hash: str | None,
        block_height: int | None,
        amount_minor: int | None,
        currency: str | None,
        verified_at: datetime,
        payload: dict[str, Any],
    ) -> None: ...


def _coerce_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


class ChainEventConsumer:
    def __init__(self, sink: VerifiedPaylinkSink) -> None:
        self._sink = sink

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        if name != CHAIN_PAYLINK_VERIFIED:
            log.debug("event_ignored", event_name=name)
            return
        # chain-event-mirror payload carries the PayLink id as ``entity_id``; accept ``pl_id`` too.
        paylink_id = payload.get("entity_id") or payload.get("pl_id")
        if not paylink_id:
            log.warning("verified_event_missing_plid", event_name=name)
            return
        raw_data = payload.get("data")
        data: dict[str, Any] = raw_data if isinstance(raw_data, dict) else {}
        ts = payload.get("timestamp")
        verified_at = (
            datetime.fromtimestamp(int(ts), tz=UTC)
            if isinstance(ts, (int, float))
            else datetime.now(UTC)
        )
        await self._sink.project_verified_paylink(
            paylink_id=str(paylink_id),
            tx_hash=payload.get("tx_hash"),
            block_height=_coerce_int(payload.get("block_height")),
            amount_minor=_coerce_int(data.get("amount")),
            currency=data.get("currency"),
            verified_at=verified_at,
            payload=payload,
        )
