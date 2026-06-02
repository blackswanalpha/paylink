"""Request/response models for the intake + delivery-view surface."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field


class ContactModel(BaseModel):
    """The destination contact, supplied on the trusted intake call (Phase-1 inline resolver)."""

    phone: str | None = None
    email: str | None = None
    locale: str | None = None


class NotificationIntakeRequest(BaseModel):
    """A domain event to fan out to channels. ``data`` feeds template ``$placeholders``.

    Recipient keys are both optional but at least one is required: ``user_id`` (→ SMS/email) and/or
    ``recipient_addr`` (→ the address-scoped in-app inbox). ``title``/``body``/``href`` optionally
    override the inbox copy the service would otherwise derive from ``event_kind`` + ``data``.
    """

    event_kind: str
    user_id: str | None = None
    recipient_addr: str | None = None
    locale: str | None = None
    data: dict[str, Any] = Field(default_factory=dict)
    contact: ContactModel | None = None
    title: str | None = None
    body: str | None = None
    href: str | None = None


class NotificationIntakeResponse(BaseModel):
    delivery_ids: list[str]


class InboxNotificationView(BaseModel):
    """One in-app inbox notification as returned by the read API (FE work07 wire shape)."""

    id: str
    kind: str  # success|info|warning|error
    title: str
    body: str | None
    href: str | None
    read: bool
    created_at: datetime


class InboxListResponse(BaseModel):
    items: list[InboxNotificationView]
    next_cursor: str | None


class DeliveryView(BaseModel):
    """Operational view of a delivery. ``recipient`` is MASKED (never the raw contact)."""

    delivery_id: str
    channel: str
    recipient: str
    event_kind: str
    status: str
    attempts: int
    last_error: str | None
    next_retry_at: datetime | None
    created_at: datetime | None
    delivered_at: datetime | None
