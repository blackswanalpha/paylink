"""Shared test doubles + helpers (imported by conftest and integration tests).

The unit/API suite runs with no Docker: an in-memory :class:`FakeRepository` mirrors
:class:`~app.db.repository.NotifyRepository`, a :class:`FakeDeliveryStore` mirrors the sync runner
store, and a spy ``enqueue`` records the delivery ids the service would hand to Celery. Real
primitives (template rendering, the backoff schedule, the pure runner, the console provider, the
inline resolver, idempotency) are all exercised against these fakes.
"""

from __future__ import annotations

import uuid
from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta
from typing import Any

from app.channels.base import SendError, SendResult
from app.config import Settings
from app.db.models import (
    DeliveryRow,
    InboxNotificationRow,
    NotificationPreferenceRow,
    TemplateRow,
)
from app.db.repository import _decode_inbox_cursor, _encode_inbox_cursor
from app.domain.models import DeliveryRecord


async def noop_commit() -> None:
    return None


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "celery_broker_url": "redis://localhost:6379/1",
        "celery_task_always_eager": True,
        "event_publisher_mode": "noop",
    }
    base.update(overrides)
    return Settings(**base)


# The 4 Phase-1 templates the migration seeds — replicated for unit tests (no DB).
def default_templates() -> list[TemplateRow]:
    rows = [
        (
            "sms.paylink_verified.en",
            "sms",
            "en",
            "Your PayLink for $amount $currency is verified. Ref: $paylink_id",
        ),
        (
            "email.paylink_verified.en",
            "email",
            "en",
            "Hi — your PayLink ($paylink_id) for $amount $currency has been verified.",
        ),
        ("sms.payment_failed.en", "sms", "en", "Payment of $amount $currency failed: $reason."),
        (
            "email.payment_failed.en",
            "email",
            "en",
            "Your payment of $amount $currency could not be completed. Reason: $reason.",
        ),
    ]
    return [
        TemplateRow(template_id=tid, channel=ch, locale=loc, body=body, version=1, active=True)
        for tid, ch, loc, body in rows
    ]


class FakeRepository:
    """In-memory NotifyRepository with matching method names/semantics."""

    def __init__(self, templates: list[TemplateRow] | None = None) -> None:
        self.deliveries: dict[uuid.UUID, DeliveryRow] = {}
        self.inbox: dict[uuid.UUID, InboxNotificationRow] = {}
        self.preferences: dict[str, NotificationPreferenceRow] = {}
        self._inbox_seq = 0
        self.templates: list[TemplateRow] = (
            templates if templates is not None else default_templates()
        )

    async def insert_delivery(self, row: DeliveryRow) -> DeliveryRow:
        self.deliveries[row.delivery_id] = row
        return row

    async def get_delivery(self, delivery_id: uuid.UUID) -> DeliveryRow | None:
        return self.deliveries.get(delivery_id)

    async def list_active_templates_for_event(self, event_kind: str) -> list[TemplateRow]:
        marker = f".{event_kind.replace('.', '_')}."
        return [t for t in self.templates if t.active and marker in t.template_id]

    async def find_delivery_by_dedupe(self, dedupe_key: str) -> DeliveryRow | None:
        for row in self.deliveries.values():
            if (row.payload or {}).get("dedupe_key") == dedupe_key:
                return row
        return None

    # --- In-app inbox (mirrors NotifyRepository semantics) -------------------------------------

    async def insert_inbox(self, row: InboxNotificationRow) -> InboxNotificationRow | None:
        for existing in self.inbox.values():
            if existing.dedupe_key == row.dedupe_key:
                return None  # dedupe — DB UNIQUE index is the arbiter in the real repo
        if row.created_at is None:
            # Stamp a strictly-increasing time so list ordering is deterministic without a DB.
            self._inbox_seq += 1
            row.created_at = datetime(2026, 1, 1, tzinfo=UTC) + timedelta(seconds=self._inbox_seq)
        if row.read is None:
            row.read = False
        self.inbox[row.notification_id] = row
        return row

    async def find_inbox_by_dedupe(self, dedupe_key: str) -> InboxNotificationRow | None:
        for row in self.inbox.values():
            if row.dedupe_key == dedupe_key:
                return row
        return None

    async def list_inbox(
        self, recipient_addr: str, *, limit: int, cursor: str | None = None
    ) -> tuple[list[InboxNotificationRow], str | None]:
        rows = [r for r in self.inbox.values() if r.recipient_addr == recipient_addr.lower()]
        rows.sort(key=lambda r: (r.created_at, r.notification_id), reverse=True)
        if cursor:
            decoded = _decode_inbox_cursor(cursor)
            if decoded is not None:
                ts, nid = decoded
                rows = [r for r in rows if (r.created_at, r.notification_id) < (ts, nid)]
        next_cursor: str | None = None
        if len(rows) > limit:
            rows = rows[:limit]
            last = rows[-1]
            next_cursor = _encode_inbox_cursor(last.created_at, last.notification_id)
        return rows, next_cursor

    async def mark_inbox_read(
        self, recipient_addr: str, notification_id: uuid.UUID
    ) -> InboxNotificationRow | None:
        row = self.inbox.get(notification_id)
        if row is None or row.recipient_addr != recipient_addr.lower():
            return None
        if not row.read:
            row.read = True
            row.read_at = datetime.now(UTC)
        return row

    async def mark_all_inbox_read(self, recipient_addr: str) -> int:
        count = 0
        for row in self.inbox.values():
            if row.recipient_addr == recipient_addr.lower() and not row.read:
                row.read = True
                row.read_at = datetime.now(UTC)
                count += 1
        return count

    # --- Notification preferences (mirrors NotifyRepository semantics) --------------------------

    async def get_preferences(self, recipient_addr: str) -> NotificationPreferenceRow | None:
        return self.preferences.get(recipient_addr.lower())

    async def upsert_preferences(
        self, recipient_addr: str, *, channels: dict[str, Any], events: dict[str, Any]
    ) -> NotificationPreferenceRow:
        addr = recipient_addr.lower()
        now = datetime.now(UTC)
        row = self.preferences.get(addr)
        if row is None:
            row = NotificationPreferenceRow(
                preference_id=uuid.uuid4(),
                recipient_addr=addr,
                channels=channels,
                events=events,
                created_at=now,
                updated_at=now,
            )
            self.preferences[addr] = row
        else:
            row.channels = channels
            row.events = events
            row.updated_at = now
        return row


class FakeDeliveryStore:
    """In-memory sync store implementing the runner's ``DeliveryStore`` Protocol."""

    def __init__(self, records: dict[uuid.UUID, DeliveryRecord] | None = None) -> None:
        self.records: dict[uuid.UUID, DeliveryRecord] = records or {}
        self.sent: list[tuple[uuid.UUID, str]] = []
        self.failed: list[tuple[uuid.UUID, int, str, datetime]] = []
        self.exhausted: list[tuple[uuid.UUID, int, str]] = []

    def add(self, record: DeliveryRecord) -> None:
        self.records[record.delivery_id] = record

    def get(self, delivery_id: uuid.UUID) -> DeliveryRecord | None:
        return self.records.get(delivery_id)

    def mark_sent(self, delivery_id: uuid.UUID, *, provider_ref: str) -> None:
        self.sent.append((delivery_id, provider_ref))
        self._set_status(delivery_id, "SENT")

    def mark_failed(
        self, delivery_id: uuid.UUID, *, attempts: int, last_error: str, next_retry_at: datetime
    ) -> None:
        self.failed.append((delivery_id, attempts, last_error, next_retry_at))
        self._set(delivery_id, status="FAILED", attempts=attempts)

    def mark_exhausted(self, delivery_id: uuid.UUID, *, attempts: int, last_error: str) -> None:
        self.exhausted.append((delivery_id, attempts, last_error))
        self._set(delivery_id, status="EXHAUSTED", attempts=attempts)

    def _set_status(self, delivery_id: uuid.UUID, status: str) -> None:
        rec = self.records.get(delivery_id)
        if rec is not None:
            self.records[delivery_id] = DeliveryRecord(
                delivery_id=rec.delivery_id,
                channel=rec.channel,
                recipient=rec.recipient,
                status=status,
                attempts=rec.attempts,
                body=rec.body,
                subject=rec.subject,
            )

    def _set(self, delivery_id: uuid.UUID, *, status: str, attempts: int) -> None:
        rec = self.records.get(delivery_id)
        if rec is not None:
            self.records[delivery_id] = DeliveryRecord(
                delivery_id=rec.delivery_id,
                channel=rec.channel,
                recipient=rec.recipient,
                status=status,
                attempts=attempts,
                body=rec.body,
                subject=rec.subject,
            )


class FakeProvider:
    """A channel provider that fails its first ``fail_times`` sends, then succeeds."""

    def __init__(self, name: str = "fake", *, fail_times: int = 0) -> None:
        self.name = name
        self._fail_times = fail_times
        self.calls = 0

    def send(self, *, to: str, body: str, subject: str | None = None) -> SendResult:
        self.calls += 1
        if self.calls <= self._fail_times:
            raise SendError(self.name, detail="forced")
        return SendResult(provider=self.name, provider_ref=f"{self.name}-{self.calls}")


class FakeChannels:
    """A ChannelLookup returning one provider for any channel."""

    def __init__(self, provider: FakeProvider | None = None) -> None:
        self.provider = provider

    def for_channel(self, channel: str) -> FakeProvider | None:
        return self.provider


def make_record(
    *,
    channel: str = "sms",
    recipient: str = "+254712345678",
    status: str = "QUEUED",
    attempts: int = 0,
    body: str = "hello",
    subject: str | None = None,
) -> DeliveryRecord:
    return DeliveryRecord(
        delivery_id=uuid.uuid4(),
        channel=channel,
        recipient=recipient,
        status=status,
        attempts=attempts,
        body=body,
        subject=subject,
    )


class EnqueueSpy:
    """Records the delivery ids the service would hand to Celery (no broker in unit tests)."""

    def __init__(self) -> None:
        self.ids: list[Any] = []

    def __call__(self, delivery_id: Any) -> None:
        self.ids.append(delivery_id)


def install_overrides(
    app: Any,
    fake_repo: FakeRepository,
    idem_store: Any,
    enqueue: Any,
) -> None:
    """Point the API's deps at in-memory fakes (consumer + repo share one FakeRepository)."""
    from app.deps import get_consumer, get_idempotency, get_repo
    from app.domain.service import NotificationService
    from app.events.consumer import NotificationEventConsumer
    from app.recipients.inline import InlineRecipientResolver
    from app.templating.registry import TemplateRegistry

    async def _consumer_override() -> AsyncIterator[Any]:
        service = NotificationService(
            repo=fake_repo,  # type: ignore[arg-type]
            registry=TemplateRegistry(fake_repo),  # type: ignore[arg-type]
            resolver=InlineRecipientResolver(),
            enqueue=enqueue,
            commit=noop_commit,
        )
        yield NotificationEventConsumer(service)

    async def _repo_override() -> AsyncIterator[Any]:
        yield fake_repo

    app.dependency_overrides[get_consumer] = _consumer_override
    app.dependency_overrides[get_repo] = _repo_override
    app.dependency_overrides[get_idempotency] = lambda: idem_store
