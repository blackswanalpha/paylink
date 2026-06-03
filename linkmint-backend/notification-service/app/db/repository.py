"""Data access for the ``notify`` schema.

Two surfaces, deliberately split by execution model:

* :class:`NotifyRepository` — **async**, used by the FastAPI intake/read path (insert deliveries,
  read templates for fan-out, dedupe lookup, fetch a delivery for the GET view).
* :class:`SyncDeliveryStore` — **sync**, used by the Celery worker's
  :class:`~app.delivery.runner.DeliveryRunner`. Each method runs in its own short transaction and
  returns ORM-decoupled :class:`~app.domain.models.DeliveryRecord` values, so the runner stays pure
  (no SQLAlchemy / Celery imports) and trivially fakeable in unit tests.
"""

from __future__ import annotations

import base64
import binascii
import uuid
from datetime import UTC, datetime
from typing import Any, cast

from sqlalchemy import and_, or_, select, update
from sqlalchemy.engine import CursorResult, Engine
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import sessionmaker

from app.db.models import (
    DeliveryRow,
    InboxNotificationRow,
    NotificationPreferenceRow,
    TemplateRow,
)
from app.db.session import make_sync_sessionmaker
from app.domain.models import EXHAUSTED, FAILED, SENT, DeliveryRecord


def _encode_inbox_cursor(created_at: datetime, notification_id: uuid.UUID) -> str:
    """Opaque keyset cursor over (created_at, notification_id) — the listing sort key."""
    raw = f"{created_at.isoformat()}|{notification_id}"
    return base64.urlsafe_b64encode(raw.encode()).decode()


def _decode_inbox_cursor(cursor: str) -> tuple[datetime, uuid.UUID] | None:
    """Decode a cursor; a malformed one is forgiving (treated as no cursor → newest page)."""
    try:
        raw = base64.urlsafe_b64decode(cursor.encode()).decode()
        ts_str, id_str = raw.split("|", 1)
        return datetime.fromisoformat(ts_str), uuid.UUID(id_str)
    except (ValueError, binascii.Error):
        return None


class NotifyRepository:
    """Async repository over a request-scoped session."""

    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def insert_delivery(self, row: DeliveryRow) -> DeliveryRow | None:
        """Insert a delivery; return ``None`` if its per-event dedupe key already exists.

        The unique index on ``payload->>'dedupe_key'`` makes the DB the arbiter (A.7 analog): a
        concurrent at-least-once redelivery that slips past the read-check loses here, and the
        caller reuses the existing delivery rather than double-sending. The savepoint keeps the
        transaction usable after the conflict.
        """
        try:
            async with self._session.begin_nested():
                self._session.add(row)
                await self._session.flush()
        except IntegrityError:
            return None
        return row

    async def get_delivery(self, delivery_id: uuid.UUID) -> DeliveryRow | None:
        return await self._session.get(DeliveryRow, delivery_id)

    async def list_active_templates_for_event(self, event_kind: str) -> list[TemplateRow]:
        """Active templates for an event (any channel/locale); registry filters + falls back."""
        # template_id is ``{channel}.{event_snake}.{locale}``. Escape LIKE metacharacters in the
        # event segment (the snake's ``_`` would otherwise be a single-char wildcard) so the match
        # is literal and a hostile/odd event_kind can't widen it.
        snake = event_kind.replace(".", "_")
        escaped = snake.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
        stmt = select(TemplateRow).where(
            TemplateRow.active.is_(True),
            TemplateRow.template_id.like(f"%.{escaped}.%", escape="\\"),
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def find_delivery_by_dedupe(self, dedupe_key: str) -> DeliveryRow | None:
        stmt = (
            select(DeliveryRow)
            .where(DeliveryRow.payload["dedupe_key"].astext == dedupe_key)
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalars().first()

    # --- In-app inbox (notification center, address-scoped) -------------------------------------

    async def insert_inbox(self, row: InboxNotificationRow) -> InboxNotificationRow | None:
        """Insert an inbox notification; return ``None`` if its dedupe_key already exists.

        Mirrors :meth:`insert_delivery` — the UNIQUE ``dedupe_key`` index is the arbiter so an
        at-least-once producer can't double-post; the savepoint keeps the transaction usable. The
        caller commits (intake shares one commit with the SMS/email fan-out).
        """
        try:
            async with self._session.begin_nested():
                self._session.add(row)
                await self._session.flush()
        except IntegrityError:
            return None
        return row

    async def find_inbox_by_dedupe(self, dedupe_key: str) -> InboxNotificationRow | None:
        stmt = (
            select(InboxNotificationRow)
            .where(InboxNotificationRow.dedupe_key == dedupe_key)
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalars().first()

    async def list_inbox(
        self, recipient_addr: str, *, limit: int, cursor: str | None = None
    ) -> tuple[list[InboxNotificationRow], str | None]:
        """Newest-first page of a recipient's inbox + an opaque keyset cursor for the next page."""
        stmt = select(InboxNotificationRow).where(
            InboxNotificationRow.recipient_addr == recipient_addr.lower()
        )
        if cursor:
            decoded = _decode_inbox_cursor(cursor)
            if decoded is not None:
                ts, nid = decoded
                # Keyset predicate for ORDER BY (created_at DESC, notification_id DESC): rows
                # strictly "older" than the cursor. Explicit OR/AND (not a row-value tuple
                # compare) keeps it portable + statically typed.
                stmt = stmt.where(
                    or_(
                        InboxNotificationRow.created_at < ts,
                        and_(
                            InboxNotificationRow.created_at == ts,
                            InboxNotificationRow.notification_id < nid,
                        ),
                    )
                )
        stmt = stmt.order_by(
            InboxNotificationRow.created_at.desc(), InboxNotificationRow.notification_id.desc()
        ).limit(limit + 1)
        rows = list((await self._session.execute(stmt)).scalars().all())
        next_cursor: str | None = None
        if len(rows) > limit:
            rows = rows[:limit]
            last = rows[-1]
            next_cursor = _encode_inbox_cursor(last.created_at, last.notification_id)
        return rows, next_cursor

    async def mark_inbox_read(
        self, recipient_addr: str, notification_id: uuid.UUID
    ) -> InboxNotificationRow | None:
        """Flip one notification to read (idempotent); ``None`` if missing or not owned."""
        row = await self._session.get(InboxNotificationRow, notification_id)
        if row is None or row.recipient_addr != recipient_addr.lower():
            return None
        if not row.read:
            row.read = True
            row.read_at = datetime.now(UTC)
        await self._session.commit()
        return row

    async def mark_all_inbox_read(self, recipient_addr: str) -> int:
        """Mark every unread notification for a recipient read; returns how many changed."""
        stmt = (
            update(InboxNotificationRow)
            .where(
                InboxNotificationRow.recipient_addr == recipient_addr.lower(),
                InboxNotificationRow.read.is_(False),
            )
            .values(read=True, read_at=datetime.now(UTC))
        )
        result = await self._session.execute(stmt)
        await self._session.commit()
        return cast("CursorResult[Any]", result).rowcount or 0

    # --- Notification preferences (address-scoped channel/event opt-outs) ------------------------

    async def get_preferences(self, recipient_addr: str) -> NotificationPreferenceRow | None:
        """Fetch a recipient's stored preferences row, or ``None`` if they've never set any."""
        stmt = (
            select(NotificationPreferenceRow)
            .where(NotificationPreferenceRow.recipient_addr == recipient_addr.lower())
            .limit(1)
        )
        return (await self._session.execute(stmt)).scalars().first()

    async def upsert_preferences(
        self, recipient_addr: str, *, channels: dict[str, Any], events: dict[str, Any]
    ) -> NotificationPreferenceRow:
        """Insert or replace a recipient's preferences (one row per ``recipient_addr``)."""
        addr = recipient_addr.lower()
        row = await self.get_preferences(addr)
        if row is None:
            row = NotificationPreferenceRow(
                preference_id=uuid.uuid4(), recipient_addr=addr, channels=channels, events=events
            )
            self._session.add(row)
        else:
            row.channels = channels
            row.events = events
            row.updated_at = datetime.now(UTC)
        await self._session.commit()
        await self._session.refresh(row)
        return row


class SyncDeliveryStore:
    """Sync delivery-row store. Implements the runner's ``DeliveryStore`` Protocol."""

    def __init__(self, engine: Engine) -> None:
        self._sm: sessionmaker = make_sync_sessionmaker(engine)

    def get(self, delivery_id: uuid.UUID) -> DeliveryRecord | None:
        with self._sm() as session:
            row = session.get(DeliveryRow, delivery_id)
            if row is None:
                return None
            payload = row.payload or {}
            return DeliveryRecord(
                delivery_id=row.delivery_id,
                channel=row.channel,
                recipient=row.recipient,
                status=row.status,
                attempts=row.attempts,
                body=str(payload.get("body", "")),
                subject=payload.get("subject"),
            )

    def mark_sent(self, delivery_id: uuid.UUID, *, provider_ref: str) -> None:
        with self._sm() as session:
            row = session.get(DeliveryRow, delivery_id)
            if row is None:
                return
            row.status = SENT
            row.delivered_at = datetime.now(UTC)
            row.next_retry_at = None
            row.last_error = None
            session.commit()

    def mark_failed(
        self,
        delivery_id: uuid.UUID,
        *,
        attempts: int,
        last_error: str,
        next_retry_at: datetime,
    ) -> None:
        with self._sm() as session:
            row = session.get(DeliveryRow, delivery_id)
            if row is None:
                return
            row.status = FAILED
            row.attempts = attempts
            row.last_error = last_error
            row.next_retry_at = next_retry_at
            session.commit()

    def mark_exhausted(self, delivery_id: uuid.UUID, *, attempts: int, last_error: str) -> None:
        with self._sm() as session:
            row = session.get(DeliveryRow, delivery_id)
            if row is None:
                return
            row.status = EXHAUSTED
            row.attempts = attempts
            row.last_error = last_error
            row.next_retry_at = None
            session.commit()
