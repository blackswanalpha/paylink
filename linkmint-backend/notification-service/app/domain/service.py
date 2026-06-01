"""The intake fan-out: one domain event → per-channel rendered deliveries (QUEUED) + enqueue.

Called only via :class:`~app.events.consumer.NotificationEventConsumer` — the single chokepoint the
HTTP intake and the future work15 bus subscriber share. Deliveries are committed BEFORE enqueue so
the worker (even in eager mode) sees a persisted row.

Per-event dedupe (distinct from Idempotency-Key) makes an at-least-once bus safe: a delivery already
created for ``(event_kind, user_id, channel, data.dedupe_id|fingerprint)`` is returned, not re-sent.
"""

from __future__ import annotations

import hashlib
import uuid
from collections.abc import Awaitable, Callable

from app.db.models import DeliveryRow
from app.db.repository import NotifyRepository
from app.domain.models import QUEUED, Channel
from app.errors import AppError, ErrorCode
from app.idempotency import fingerprint
from app.logging import get_logger
from app.recipients.base import RecipientResolver
from app.redaction import safe_data_keys
from app.templating.registry import TemplateRegistry
from app.templating.render import render

log = get_logger("notify.service")

# The enqueue seam: hand a delivery id to Celery (apply_async). Injected so tests use a spy.
Enqueue = Callable[[uuid.UUID], None]


class NotificationService:
    def __init__(
        self,
        *,
        repo: NotifyRepository,
        registry: TemplateRegistry,
        resolver: RecipientResolver,
        enqueue: Enqueue,
        commit: Callable[[], Awaitable[None]],
    ) -> None:
        self._repo = repo
        self._registry = registry
        self._resolver = resolver
        self._enqueue = enqueue
        self._commit = commit

    @staticmethod
    def _dedupe_key(
        event_kind: str, user_id: uuid.UUID, channel: Channel, data: dict[str, object]
    ) -> str:
        base = data.get("dedupe_id") or fingerprint(data)
        raw = f"{event_kind}|{user_id}|{channel.value}|{base}"
        return hashlib.sha256(raw.encode()).hexdigest()

    async def intake(
        self,
        *,
        event_kind: str,
        user_id: uuid.UUID,
        locale: str | None,
        data: dict[str, object],
        contact: dict[str, object] | None,
    ) -> list[uuid.UUID]:
        recipient = await self._resolver.resolve(user_id, contact)
        templates = await self._registry.resolve_for_event(event_kind, locale or recipient.locale)
        if not templates:
            raise AppError(
                ErrorCode.TEMPLATE_NOT_FOUND,
                f"no active template for event {event_kind}",
                details={"event_kind": event_kind},
            )

        created: list[uuid.UUID] = []
        to_enqueue: list[uuid.UUID] = []
        for channel, template in templates.items():
            address = recipient.address_for(channel)
            if not address:
                log.info("channel_skipped_no_contact", channel=channel.value, event_kind=event_kind)
                continue

            dedupe_key = self._dedupe_key(event_kind, user_id, channel, data)
            existing = await self._repo.find_delivery_by_dedupe(dedupe_key)
            if existing is not None:
                created.append(existing.delivery_id)  # idempotent — already created, do not re-send
                continue

            subject = (
                self._registry.subject_for(event_kind, data) if channel is Channel.EMAIL else None
            )
            payload = {
                "body": render(template.body, data),
                "subject": subject,
                "dedupe_key": dedupe_key,
                "data": safe_data_keys(data),
            }
            delivery_id = uuid.uuid4()
            inserted = await self._repo.insert_delivery(
                DeliveryRow(
                    delivery_id=delivery_id,
                    channel=channel.value,
                    recipient=address,
                    event_kind=event_kind,
                    payload=payload,
                    status=QUEUED,
                    attempts=0,
                )
            )
            if inserted is None:
                # Lost a concurrent race on the dedupe key — reuse the winner, never re-send.
                winner = await self._repo.find_delivery_by_dedupe(dedupe_key)
                if winner is not None:
                    created.append(winner.delivery_id)
                continue
            created.append(delivery_id)
            to_enqueue.append(delivery_id)

        await self._commit()
        for delivery_id in to_enqueue:
            self._enqueue(delivery_id)  # after commit: the worker must see a persisted row
        return created
