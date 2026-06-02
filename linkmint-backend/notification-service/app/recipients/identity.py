"""Deferred recipient resolver: fetch the contact from identity-service at send time.

The production-correct, PII-free path — the bus event carries only ``user_id`` and notification-
service looks the contact up over the trusted-network ``/internal`` surface (ADR-009 X-Internal-
Token). Config-gated (``NOTIFY_RECIPIENT_RESOLVER=identity``); off by default. A lookup miss/error
yields a contactless :class:`Recipient` (the service has nothing to address — logged, not fatal).

NOTE: identity-service does not yet expose ``/internal/contacts/{id}`` — wiring that endpoint is the
work15-era follow-up. This resolver is the seam that consumes it.
"""

from __future__ import annotations

import uuid
from typing import Any

import httpx

from app.logging import get_logger
from app.recipients.base import Recipient

log = get_logger("notify.recipients")


class IdentityRecipientResolver:
    def __init__(
        self, client: httpx.AsyncClient, *, base_url: str, internal_token: str | None = None
    ) -> None:
        self._client = client
        self._base = base_url.rstrip("/")
        self._token = internal_token

    async def resolve(self, user_id: uuid.UUID, contact: dict[str, Any] | None) -> Recipient:
        headers = {"X-Internal-Token": self._token} if self._token else {}
        try:
            resp = await self._client.get(
                f"{self._base}/internal/contacts/{user_id}", headers=headers
            )
        except httpx.HTTPError as exc:
            log.warning("identity_contact_lookup_failed", user_id=str(user_id), error=str(exc))
            return Recipient(user_id=user_id)
        if resp.status_code >= 400:
            log.warning(
                "identity_contact_lookup_miss", user_id=str(user_id), status=resp.status_code
            )
            return Recipient(user_id=user_id)
        data = resp.json()
        return Recipient(
            user_id=user_id,
            phone=data.get("phone"),
            email=data.get("email"),
            locale=data.get("locale"),
        )
