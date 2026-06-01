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

import uuid
from datetime import UTC, datetime

from sqlalchemy import select
from sqlalchemy.engine import Engine
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import sessionmaker

from app.db.models import DeliveryRow, TemplateRow
from app.db.session import make_sync_sessionmaker
from app.domain.models import EXHAUSTED, FAILED, SENT, DeliveryRecord


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
