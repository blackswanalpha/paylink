"""ORM models for the ``notify`` schema (mirrors backendfeatures.md §2.7 DDL).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy resolves
``Mapped[...]`` annotations at class-creation time and is most robust with real (non-stringized)
types.

``notify.webhooks`` is **forward-schema only** (Phase 2: merchant webhook registration + HMAC
delivery). Phase 1 reads/writes ``deliveries`` + ``templates`` only.
"""

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import (
    Boolean,
    DateTime,
    ForeignKey,
    Integer,
    Text,
    text,
)
from sqlalchemy.dialects.postgresql import ARRAY, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class WebhookRow(Base):
    """Merchant webhook registration — **Phase 2 forward-schema** (unused in Phase 1)."""

    __tablename__ = "webhooks"
    __mapper_args__ = {"eager_defaults": True}

    webhook_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), primary_key=True, default=uuid.uuid4
    )
    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    url: Mapped[str] = mapped_column(Text, nullable=False)
    events: Mapped[list[str]] = mapped_column(ARRAY(Text), nullable=False)
    secret: Mapped[str] = mapped_column(Text, nullable=False)  # KMS-encrypted (Phase 2)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # ACTIVE|PAUSED|REVOKED
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class DeliveryRow(Base):
    """One delivery attempt-group — the durable system-of-record for retries.

    ``payload`` carries the rendered message + dedupe key (``{body, subject?, dedupe_key, data}``).
    ``recipient`` legitimately stores the real phone/email (§2.7); it is masked in logs + responses.
    ``attempts`` increments on FAILURE only (a clean first send leaves it at 0).
    """

    __tablename__ = "deliveries"
    __mapper_args__ = {"eager_defaults": True}

    delivery_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), primary_key=True, default=uuid.uuid4
    )
    # Nullable in Phase 1 (webhook deliveries are Phase 2); in-schema FK is fine (same schema).
    webhook_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("webhooks.webhook_id"), nullable=True
    )
    channel: Mapped[str] = mapped_column(Text, nullable=False)  # sms|email|push|webhook
    recipient: Mapped[str] = mapped_column(Text, nullable=False)
    event_kind: Mapped[str] = mapped_column(Text, nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # QUEUED|SENT|FAILED|EXHAUSTED
    attempts: Mapped[int] = mapped_column(Integer, nullable=False, server_default=text("0"))
    last_error: Mapped[str | None] = mapped_column(Text, nullable=True)
    next_retry_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    delivered_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class TemplateRow(Base):
    """A channel/locale message template, addressed ``{channel}.{event_snake}.{locale}``."""

    __tablename__ = "templates"
    __mapper_args__ = {"eager_defaults": True}

    template_id: Mapped[str] = mapped_column(Text, primary_key=True)  # sms.paylink_verified.en
    channel: Mapped[str] = mapped_column(Text, nullable=False)
    locale: Mapped[str] = mapped_column(Text, nullable=False)
    body: Mapped[str] = mapped_column(Text, nullable=False)
    version: Mapped[int] = mapped_column(Integer, nullable=False)
    active: Mapped[bool] = mapped_column(Boolean, nullable=False, server_default=text("true"))


class InboxNotificationRow(Base):
    """An in-app inbox notification, scoped to a recipient ADDRESS (the creator/merchant address the
    gateway injects as ``X-Creator-Addr``) — distinct from the UUID-keyed SMS/email ``deliveries``.

    This backs the web app's notification center (FE work07): the read API filters by
    ``recipient_addr`` (ownership), and ``dedupe_key`` (UNIQUE) makes the write path safe for an
    at-least-once producer the same way ``deliveries`` is. ``read`` flips when the user opens it.
    """

    __tablename__ = "inbox_notifications"
    __mapper_args__ = {"eager_defaults": True}

    notification_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), primary_key=True, default=uuid.uuid4
    )
    recipient_addr: Mapped[str] = mapped_column(Text, nullable=False)  # lowercased creator address
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # success|info|warning|error
    title: Mapped[str] = mapped_column(Text, nullable=False)
    body: Mapped[str | None] = mapped_column(Text, nullable=True)
    href: Mapped[str | None] = mapped_column(Text, nullable=True)
    event_kind: Mapped[str | None] = mapped_column(Text, nullable=True)
    dedupe_key: Mapped[str] = mapped_column(Text, nullable=False)
    read: Mapped[bool] = mapped_column(
        Boolean, nullable=False, default=False, server_default=text("false")
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    read_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class NotificationPreferenceRow(Base):
    """A recipient's notification preferences, scoped to the same creator/merchant ADDRESS the inbox
    uses (``X-Creator-Addr``, lowercased). Two flag maps drive the fan-out (``app.domain.service``):
    ``channels`` (``in_app``/``email``/``sms``) and ``events`` (the ``paylink.*``/``payment.*``).

    Opt-out semantics: a missing recipient row — or a missing key within a map — means *enabled*, so
    a brand-new recipient gets everything until they turn something off. One row per recipient
    (``recipient_addr`` UNIQUE); the API upserts it.
    """

    __tablename__ = "notification_preferences"
    __mapper_args__ = {"eager_defaults": True}

    preference_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), primary_key=True, default=uuid.uuid4
    )
    recipient_addr: Mapped[str] = mapped_column(Text, nullable=False, unique=True)  # lowercased
    channels: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    events: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
