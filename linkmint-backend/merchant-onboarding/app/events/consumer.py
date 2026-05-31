"""Inbound event consumer seam.

merchant-onboarding consumes ``compliance.kyb.passed`` / ``compliance.kyb.failed`` (Know Your
Business — distinct from per-user KYC) and ``admin.override.suspend`` / ``admin.override.reinstate``
(from admin-backoffice). The Kafka/SQS transport is delivered by work15/16; until then this is a
thin, typed handler the future subscriber will call (and which the integration test drives directly
via ``MerchantsService.decide``).

Every inbound event maps to a single guarded ``MerchantsService.decide`` call, so the manual-review
``/internal`` endpoint and the consumer share one state-machine path (mirrors identity's
``KycConsumer``). Unknown events / missing merchant id → log + no-op.
"""

from __future__ import annotations

import uuid
from typing import Any

from app.domain.merchants_service import MerchantsService
from app.domain.models import ReviewDecision
from app.logging import get_logger

log = get_logger("merchant.consumer")

KYB_PASSED = "compliance.kyb.passed"
KYB_FAILED = "compliance.kyb.failed"
ADMIN_OVERRIDE_SUSPEND = "admin.override.suspend"
ADMIN_OVERRIDE_REINSTATE = "admin.override.reinstate"

_DECISION_BY_EVENT: dict[str, ReviewDecision] = {
    KYB_PASSED: ReviewDecision.APPROVE,
    KYB_FAILED: ReviewDecision.REJECT,
    ADMIN_OVERRIDE_SUSPEND: ReviewDecision.SUSPEND,
    ADMIN_OVERRIDE_REINSTATE: ReviewDecision.REINSTATE,
}


class MerchantEventConsumer:
    def __init__(self, merchants: MerchantsService) -> None:
        self._merchants = merchants

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        decision = _DECISION_BY_EVENT.get(name)
        if decision is None:
            log.warning("merchant_event_unknown", event_name=name)
            return
        merchant_id_raw = payload.get("merchant_id")
        if not merchant_id_raw:
            log.warning("merchant_event_missing_id", event_name=name)
            return
        try:
            merchant_id = uuid.UUID(str(merchant_id_raw))
        except ValueError:
            # A malformed id is a no-op (not a crash) — the inbound bus may deliver junk; the
            # documented contract is "missing/bad merchant id → log + no-op".
            log.warning("merchant_event_bad_id", event_name=name, merchant_id=str(merchant_id_raw))
            return
        reason = payload.get("reason")
        await self._merchants.decide(
            merchant_id=merchant_id,
            decision=decision,
            reason=str(reason) if reason is not None else None,
        )
