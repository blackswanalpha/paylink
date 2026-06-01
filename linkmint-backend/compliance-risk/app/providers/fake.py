"""In-process KYC provider — the MVP default AND the test double (same Protocol as the HTTP impl).

``start`` mints a synthetic session id + a stub hosted-verification URL. ``parse_callback`` reads a
minimal normalized body (``user_id``, ``status`` ∈ {passed, failed}, optional ``tier``) — the shape
the integration smoke test and docs use. No network, no PII.
"""

from __future__ import annotations

import uuid
from typing import Any

from app.providers.base import CallbackResult, StartResult


class FakeKycProvider:
    name = "stub"

    async def start(self, user_id: uuid.UUID, tier: int) -> StartResult:
        sid = uuid.uuid4().hex
        return StartResult(session_id=sid, provider_url=f"https://kyc.stub.local/s/{sid}")

    def parse_callback(self, body: dict[str, Any]) -> CallbackResult:
        user_id = uuid.UUID(str(body["user_id"]))
        status = str(body["status"]).lower()
        passed = status == "passed"
        tier_granted = int(body.get("tier", 2))
        # A stable provider_ref: the callback's session_id when present, else derived from the user.
        provider_ref = str(body.get("session_id") or f"stub-{user_id}")
        return CallbackResult(
            user_id=user_id,
            passed=passed,
            tier_granted=tier_granted,
            provider_ref=provider_ref,
            metadata={
                "status": status,
                "session_id": str(body.get("session_id", "")),
                "tier": tier_granted,
            },
        )
