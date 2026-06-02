"""Neutral domain dataclasses shared across the app + the (Celery-free) delivery runner.

Kept dependency-free (no SQLAlchemy, no Celery, no FastAPI) so both the async web side and the sync
worker side can import them.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from enum import StrEnum

# Delivery lifecycle states (mirror §2.7).
QUEUED = "QUEUED"
SENT = "SENT"
FAILED = "FAILED"
EXHAUSTED = "EXHAUSTED"


class Channel(StrEnum):
    """Channels addressable in Phase 1. (push/webhook are Phase-2 strings, not members.)"""

    SMS = "sms"
    EMAIL = "email"


@dataclass(frozen=True)
class RenderedMessage:
    """A template rendered for one channel + recipient, ready to enqueue."""

    channel: Channel
    recipient: str
    body: str
    subject: str | None = None


@dataclass(frozen=True)
class DeliveryRecord:
    """The delivery runner's read-only view of a ``notify.deliveries`` row (ORM-decoupled)."""

    delivery_id: uuid.UUID
    channel: str
    recipient: str
    status: str
    attempts: int
    body: str
    subject: str | None
