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
    """A domain event to fan out to channels. ``data`` feeds template ``$placeholders``."""

    event_kind: str
    user_id: str
    locale: str | None = None
    data: dict[str, Any] = Field(default_factory=dict)
    contact: ContactModel | None = None


class NotificationIntakeResponse(BaseModel):
    delivery_ids: list[str]


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
