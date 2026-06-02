"""Inbound event consumer — the work15 bus chokepoint.

``handle(name, payload)`` is the single entry point both the trusted-network HTTP intake
(``POST /v1/notifications``) and the future Kafka/SQS subscriber call (mirrors compliance-risk's
``ComplianceEventConsumer.handle``). Until work15's transport lands, tests drive this directly.

Phase-1 routes the ``paylink.created`` / ``paylink.verified`` / ``paylink.cancelled`` /
``payment.failed`` events. An event needs at least one recipient key: a ``recipient_addr`` (→ the
address-scoped in-app inbox, FE work07) and/or a UUID ``user_id`` (→ the SMS/email fan-out). Unknown
event / no usable recipient → log + no-op (``[]``) — the documented "unknown/bad → log + no-op"
contract (an at-least-once bus may deliver junk).

Payloads carry ids/metadata only; the destination ``contact`` (when present) arrives ONLY on the
trusted HTTP intake call and is never re-emitted or logged raw.
"""

from __future__ import annotations

import uuid
from typing import Any

from app.domain.service import NotificationService
from app.logging import get_logger

log = get_logger("notify.consumer")

PAYLINK_CREATED = "paylink.created"
PAYLINK_VERIFIED = "paylink.verified"
PAYLINK_CANCELLED = "paylink.cancelled"
PAYMENT_FAILED = "payment.failed"
KNOWN_EVENTS = frozenset({PAYLINK_CREATED, PAYLINK_VERIFIED, PAYLINK_CANCELLED, PAYMENT_FAILED})


class NotificationEventConsumer:
    def __init__(self, service: NotificationService) -> None:
        self._service = service

    async def handle(self, name: str, payload: dict[str, Any]) -> list[uuid.UUID]:
        if name not in KNOWN_EVENTS:
            log.warning("event_unknown", event_name=name)
            return []

        # SMS/email recipient (optional). A malformed user_id doesn't abort the inbox path.
        user_id: uuid.UUID | None = None
        user_id_raw = payload.get("user_id")
        if user_id_raw:
            try:
                user_id = uuid.UUID(str(user_id_raw))
            except ValueError:
                log.warning("event_bad_user", event_name=name)

        # In-app inbox recipient (optional) — the lowercased creator/merchant address.
        recipient_addr_raw = payload.get("recipient_addr")
        recipient_addr = (
            recipient_addr_raw
            if isinstance(recipient_addr_raw, str) and recipient_addr_raw
            else None
        )

        if recipient_addr is None and user_id is None:
            log.warning("event_missing_recipient", event_name=name)
            return []

        return await self._service.intake(
            event_kind=name,
            data=payload.get("data") or {},
            user_id=user_id,
            recipient_addr=recipient_addr,
            locale=payload.get("locale"),
            contact=payload.get("contact"),
            title=payload.get("title"),
            body=payload.get("body"),
            href=payload.get("href"),
        )
