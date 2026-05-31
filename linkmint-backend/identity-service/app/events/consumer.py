"""Inbound event consumer seam.

identity-service consumes ``compliance.kyc.passed`` / ``compliance.kyc.failed`` to update a user's
``kyc_tier``. The Kafka/SQS transport is delivered by work15/16; until then this is a thin, typed
handler the future subscriber will call (and which the integration test drives directly via
``UsersService.set_kyc_tier``).
"""

from __future__ import annotations

import uuid
from typing import Any

from app.domain.users_service import UsersService
from app.logging import get_logger

log = get_logger("identity.consumer")

KYC_PASSED = "compliance.kyc.passed"
KYC_FAILED = "compliance.kyc.failed"


class KycConsumer:
    def __init__(self, users: UsersService) -> None:
        self._users = users

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        user_id_raw = payload.get("user_id")
        if not user_id_raw:
            log.warning("kyc_event_missing_user", event_name=name)
            return
        user_id = uuid.UUID(str(user_id_raw))
        if name == KYC_PASSED:
            await self._users.set_kyc_tier(user_id, int(payload.get("tier", 1)))
        elif name == KYC_FAILED:
            await self._users.set_kyc_tier(user_id, 0)
        else:
            log.warning("kyc_event_unknown", event_name=name)
