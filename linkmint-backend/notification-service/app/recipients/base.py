"""Recipient resolution seam — how a ``user_id`` becomes a destination contact.

This is the PII seam. Other services emit events carrying ids/metadata only; the destination phone/
email must NOT ride durable bus payloads. Two implementations:

* :class:`~app.recipients.inline.InlineRecipientResolver` (Phase-1 default) — reads ``contact`` off
  the trusted intake call (a caller that already holds it).
* :class:`~app.recipients.identity.IdentityRecipientResolver` (deferred, config-gated) — fetches the
  contact from identity-service at send time, so events/intake carry only ``user_id``.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from typing import Any, Protocol

from app.domain.models import Channel


@dataclass(frozen=True)
class Recipient:
    user_id: uuid.UUID
    phone: str | None = None
    email: str | None = None
    locale: str | None = None

    def address_for(self, channel: Channel) -> str | None:
        if channel is Channel.SMS:
            return self.phone
        if channel is Channel.EMAIL:
            return self.email
        return None


class RecipientResolver(Protocol):
    async def resolve(self, user_id: uuid.UUID, contact: dict[str, Any] | None) -> Recipient: ...
