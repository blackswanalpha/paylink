"""httpx-async KYC provider — the production drop-in (Jumio / Smile Identity / Onfido).

``start`` POSTs to the vendor's hosted-session endpoint; a 404/non-2xx raises :class:`UpstreamError`
(→ ``PROVIDER_ERROR`` 502 at the route). ``parse_callback`` normalizes a vendor callback body — the
per-vendor field mapping is a documented follow-up; here it reads a generic shape so the seam is
exercisable. Real per-vendor HMAC secrets live in ``COMPLIANCE_CALLBACK_SECRETS``.
"""

from __future__ import annotations

import uuid
from typing import Any

import httpx

from app.providers.base import CallbackResult, StartResult, UpstreamError


class HttpKycProvider:
    def __init__(self, name: str, client: httpx.AsyncClient, base_url: str) -> None:
        self.name = name
        self._client = client
        self._base = base_url.rstrip("/")

    async def start(self, user_id: uuid.UUID, tier: int) -> StartResult:
        try:
            resp = await self._client.post(
                f"{self._base}/sessions",
                json={"user_id": str(user_id), "tier": tier},
            )
        except httpx.HTTPError as exc:
            raise UpstreamError(self.name) from exc
        if resp.status_code >= 400:
            raise UpstreamError(self.name, status=resp.status_code)
        data = resp.json()
        return StartResult(
            session_id=str(data["session_id"]),
            provider_url=str(data["provider_url"]),
        )

    def parse_callback(self, body: dict[str, Any]) -> CallbackResult:
        user_id = uuid.UUID(str(body["user_id"]))
        status = str(body.get("status", "")).lower()
        passed = status in {"passed", "approved", "verified"}
        tier_granted = int(body.get("tier", 1))
        return CallbackResult(
            user_id=user_id,
            passed=passed,
            tier_granted=tier_granted,
            provider_ref=str(body.get("session_id", "")),
            metadata=dict(body),
        )
