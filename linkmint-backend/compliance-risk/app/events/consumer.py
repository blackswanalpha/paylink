"""Inbound event consumer seam.

compliance-risk consumes ``payment.initiated`` to feed its activity ledger (the windowed velocity
counts + cumulative-amount AML sum the risk engine reads). ``paylink.requested`` is handled
*synchronously* via ``/v1/risk/evaluate`` from paylink-service for above-threshold amounts (the
documented sync seam), so it is a no-op here — recorded only as a log line. The Kafka/SQS transport
is delivered by work15/16; until then this is a thin, typed handler the future subscriber will call
(and which the integration test can drive directly).

Payloads carry ids/amounts metadata only — NEVER raw PII. Unknown events / missing fields → log +
no-op (the inbound bus may deliver junk; the documented contract is "unknown/bad → log + no-op").
"""

from __future__ import annotations

import uuid
from decimal import Decimal, InvalidOperation
from typing import Any, Protocol

from app.logging import get_logger

log = get_logger("compliance.consumer")

PAYMENT_INITIATED = "payment.initiated"
PAYLINK_REQUESTED = "paylink.requested"


class ActivityRecorder(Protocol):
    async def record_activity(
        self,
        *,
        user_id: uuid.UUID,
        action: str,
        amount: Decimal | None,
        currency: str | None,
    ) -> None: ...


def _parse_amount(raw: Any) -> Decimal | None:
    if raw is None:
        return None
    try:
        return Decimal(str(raw))
    except (InvalidOperation, ValueError):
        return None


class ComplianceEventConsumer:
    def __init__(self, risk: ActivityRecorder) -> None:
        self._risk = risk

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        if name == PAYMENT_INITIATED:
            await self._handle_payment_initiated(payload)
            return
        if name == PAYLINK_REQUESTED:
            # Handled synchronously via /v1/risk/evaluate (paylink-service calls us). Noted only.
            log.info("paylink_requested_sync_seam", event_name=name)
            return
        log.warning("compliance_event_unknown", event_name=name)

    async def _handle_payment_initiated(self, payload: dict[str, Any]) -> None:
        user_id_raw = payload.get("user_id")
        if not user_id_raw:
            log.warning("compliance_event_missing_user", event_name=PAYMENT_INITIATED)
            return
        try:
            user_id = uuid.UUID(str(user_id_raw))
        except ValueError:
            log.warning(
                "compliance_event_bad_user",
                event_name=PAYMENT_INITIATED,
                user_id=str(user_id_raw),
            )
            return
        await self._risk.record_activity(
            user_id=user_id,
            action=PAYMENT_INITIATED,
            amount=_parse_amount(payload.get("amount")),
            currency=str(payload["currency"]) if payload.get("currency") else None,
        )
