"""Inbound event consumer — keeps the local merchant-pricing tier cache in sync.

fee-pricing-service consumes the ``merchant`` topic (merchant-onboarding is the producer):
``merchant.onboarded`` seeds a merchant at the default tier (and captures its ``org_id`` for the
read gate); ``merchant.fee_tier.changed`` updates the tier. Delivery is at-least-once, so the
handler is idempotent (the upsert is). Unknown tiers fall back to the default with a warning — the
consumer must never reject an event (work10 only emits standard/startup/enterprise; work21's set is
a superset). Unknown events / missing fields → log + no-op.
"""

from __future__ import annotations

from typing import Any, Protocol

from app.domain.models import DEFAULT_TIER
from app.logging import get_logger

log = get_logger("pricing.consumer")

MERCHANT_ONBOARDED = "merchant.onboarded"  # {merchant_id, org_id, country, type, status}
MERCHANT_FEE_TIER_CHANGED = "merchant.fee_tier.changed"  # {merchant_id, tier}


class MerchantPricingSink(Protocol):
    async def upsert_merchant_pricing(
        self, *, merchant_id: str, tier: str, source: str, org_id: str | None
    ) -> None: ...


class MerchantPricingEventConsumer:
    def __init__(self, pricing: MerchantPricingSink) -> None:
        self._pricing = pricing

    async def handle(self, name: str, payload: dict[str, Any]) -> None:
        if name == MERCHANT_ONBOARDED:
            merchant_id = payload.get("merchant_id")
            if not merchant_id:
                log.warning("merchant_event_missing_id", event_name=name)
                return
            await self._pricing.upsert_merchant_pricing(
                merchant_id=str(merchant_id),
                tier=DEFAULT_TIER,
                source="onboarded",
                org_id=str(payload["org_id"]) if payload.get("org_id") else None,
            )
        elif name == MERCHANT_FEE_TIER_CHANGED:
            merchant_id = payload.get("merchant_id")
            tier = payload.get("tier")
            if not merchant_id or not tier:
                log.warning("merchant_event_missing_fields", event_name=name)
                return
            await self._pricing.upsert_merchant_pricing(
                merchant_id=str(merchant_id),
                tier=str(tier),
                source="fee_tier.changed",
                org_id=None,
            )
        else:
            log.debug("event_ignored", event_name=name)
