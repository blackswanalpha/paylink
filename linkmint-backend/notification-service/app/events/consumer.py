"""Inbound event consumer — the work15 bus chokepoint.

``handle(name, payload)`` is the single entry point both the trusted-network HTTP intake
(``POST /v1/notifications``) and the future Kafka/SQS subscriber call (mirrors compliance-risk's
``ComplianceEventConsumer.handle``). Until work15's transport lands, tests drive this directly.

Phase-1 routes ``paylink.verified`` + ``payment.failed``. Unknown event / missing or malformed
``user_id`` → log + no-op (``[]``) — the documented "unknown/bad → log + no-op" contract (an
at-least-once bus may deliver junk).

Payloads carry ids/metadata only; the destination ``contact`` (when present) arrives ONLY on the
trusted HTTP intake call and is never re-emitted or logged raw.
"""

from __future__ import annotations

import uuid
from typing import Any

from app.domain.service import NotificationService
from app.logging import get_logger

log = get_logger("notify.consumer")

PAYLINK_VERIFIED = "paylink.verified"
PAYMENT_FAILED = "payment.failed"
KNOWN_EVENTS = frozenset({PAYLINK_VERIFIED, PAYMENT_FAILED})


class NotificationEventConsumer:
    def __init__(self, service: NotificationService) -> None:
        self._service = service

    async def handle(self, name: str, payload: dict[str, Any]) -> list[uuid.UUID]:
        if name not in KNOWN_EVENTS:
            log.warning("event_unknown", event_name=name)
            return []

        user_id_raw = payload.get("user_id")
        if not user_id_raw:
            log.warning("event_missing_user", event_name=name)
            return []
        try:
            user_id = uuid.UUID(str(user_id_raw))
        except ValueError:
            log.warning("event_bad_user", event_name=name)
            return []

        data = payload.get("data") or {}
        contact = payload.get("contact")
        return await self._service.intake(
            event_kind=name,
            user_id=user_id,
            locale=payload.get("locale"),
            data=data,
            contact=contact,
        )
