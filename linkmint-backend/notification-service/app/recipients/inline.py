"""Phase-1 recipient resolver: the contact is supplied on the trusted intake call.

The caller (a trusted service that already legitimately holds the contact) passes
``contact={phone?, email?, locale?}`` to ``POST /v1/notifications``. The contact is never placed in
a durable bus payload and never logged raw. A missing field simply means that channel can't be
addressed (the service skips it — not an error).
"""

from __future__ import annotations

import uuid
from typing import Any

from app.recipients.base import Recipient


class InlineRecipientResolver:
    async def resolve(self, user_id: uuid.UUID, contact: dict[str, Any] | None) -> Recipient:
        contact = contact or {}
        phone = contact.get("phone")
        email = contact.get("email")
        locale = contact.get("locale")
        return Recipient(
            user_id=user_id,
            phone=str(phone) if phone else None,
            email=str(email) if email else None,
            locale=str(locale) if locale else None,
        )
