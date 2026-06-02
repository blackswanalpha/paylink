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

from linkmint_idempotency import fingerprint

from app.db.models import DeliveryRow, InboxNotificationRow
from app.db.repository import NotifyRepository
from app.domain.models import QUEUED, Channel
from app.errors import AppError, ErrorCode
from app.logging import get_logger
from app.recipients.base import RecipientResolver
from app.redaction import safe_data_keys
from app.templating.registry import TemplateRegistry
from app.templating.render import render

log = get_logger("notify.service")

# The enqueue seam: hand a delivery id to Celery (apply_async). Injected so tests use a spy.
Enqueue = Callable[[uuid.UUID], None]

# In-app inbox severity by event (the read API + web center colour-code by `kind`).
_INBOX_KIND: dict[str, str] = {
    "paylink.created": "info",
    "paylink.verified": "success",
    "paylink.cancelled": "warning",
    "payment.failed": "error",
}


def _inbox_presentation(event_kind: str, data: dict[str, object]) -> tuple[str, str, str]:
    """Map a domain event + its data to (kind, title, body) for the in-app inbox.

    Callers may override title/body explicitly on the intake; this is the fallback copy so an
    address-scoped event always yields a sensible, PII-free notification.
    """
    kind = _INBOX_KIND.get(event_kind, "info")
    pl = str(data.get("pl_id") or data.get("paylink_id") or "")
    amount = data.get("amount")
    currency = str(data.get("currency") or "")
    amt = f"{amount} {currency}".strip() if amount is not None else ""
    if event_kind == "paylink.created":
        return (
            kind,
            "PayLink created",
            (f"PayLink {pl} for {amt} is ready to share." if amt else f"PayLink {pl} created."),
        )
    if event_kind == "paylink.verified":
        return (
            kind,
            "PayLink settled",
            (
                f"PayLink {pl} for {amt} was verified on-chain."
                if amt
                else f"PayLink {pl} verified."
            ),
        )
    if event_kind == "paylink.cancelled":
        return kind, "PayLink cancelled", f"PayLink {pl} was cancelled."
    if event_kind == "payment.failed":
        reason = str(data.get("reason") or "unknown reason")
        return (
            kind,
            "Payment failed",
            (f"A payment of {amt} failed: {reason}." if amt else f"A payment failed: {reason}."),
        )
    return kind, event_kind, ""


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

    async def _write_inbox(
        self,
        *,
        event_kind: str,
        recipient_addr: str,
        data: dict[str, object],
        title: str | None,
        body: str | None,
        href: str | None,
    ) -> uuid.UUID | None:
        """Persist one address-scoped in-app inbox notification (idempotent by dedupe_key)."""
        kind, default_title, default_body = _inbox_presentation(event_kind, data)
        dedupe_base = data.get("dedupe_id") or fingerprint(data)
        raw = f"inbox|{event_kind}|{recipient_addr.lower()}|{dedupe_base}"
        dedupe_key = hashlib.sha256(raw.encode()).hexdigest()

        notification_id = uuid.uuid4()
        inserted = await self._repo.insert_inbox(
            InboxNotificationRow(
                notification_id=notification_id,
                recipient_addr=recipient_addr.lower(),
                kind=kind,
                title=title or default_title,
                body=body or default_body or None,
                href=href,
                event_kind=event_kind,
                dedupe_key=dedupe_key,
                read=False,
            )
        )
        if inserted is None:
            # Already posted (at-least-once producer) — reuse the existing notification.
            existing = await self._repo.find_inbox_by_dedupe(dedupe_key)
            return existing.notification_id if existing is not None else None
        return notification_id

    async def intake(
        self,
        *,
        event_kind: str,
        data: dict[str, object],
        user_id: uuid.UUID | None = None,
        recipient_addr: str | None = None,
        locale: str | None = None,
        contact: dict[str, object] | None = None,
        title: str | None = None,
        body: str | None = None,
        href: str | None = None,
    ) -> list[uuid.UUID]:
        """Fan an event out to its surfaces. Two independent paths share one commit:

        * ``recipient_addr`` → an in-app inbox notification (address-scoped; no template needed).
        * ``user_id`` → the Phase-1 SMS/email delivery fan-out (templated, contact-resolved).

        An intake may carry either or both. At least one recipient key is required by the consumer.
        """
        created: list[uuid.UUID] = []
        to_enqueue: list[uuid.UUID] = []

        if recipient_addr:
            inbox_id = await self._write_inbox(
                event_kind=event_kind,
                recipient_addr=recipient_addr,
                data=data,
                title=title,
                body=body,
                href=href,
            )
            if inbox_id is not None:
                created.append(inbox_id)

        if user_id is None:
            await self._commit()
            return created

        recipient = await self._resolver.resolve(user_id, contact)
        templates = await self._registry.resolve_for_event(event_kind, locale or recipient.locale)
        if not templates:
            raise AppError(
                ErrorCode.TEMPLATE_NOT_FOUND,
                f"no active template for event {event_kind}",
                details={"event_kind": event_kind},
            )

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
