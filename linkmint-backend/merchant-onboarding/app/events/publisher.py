"""Domain-event publisher seam.

Events are referenced by their **logical name** (the backendfeatures.md taxonomy). The concrete
Kafka/SQS transport (ADR-004) is delivered by **work15**; until then a publisher just logs or
no-ops. The durable record of every event is the ``merchant.merchant_events`` table, written
in-transaction by the service (so work15 can drain it as an outbox).

INVARIANT: payloads NEVER carry plaintext bank-account details — only ids, status, and metadata.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by merchant-onboarding (backendfeatures.md §2.10).
MERCHANT_ONBOARDED = "merchant.onboarded"
MERCHANT_VERIFIED = "merchant.verified"
MERCHANT_REJECTED = "merchant.rejected"
MERCHANT_SUSPENDED = "merchant.suspended"
BANK_ACCOUNT_ADDED = "merchant.bank_account.added"
BANK_ACCOUNT_VERIFIED = "merchant.bank_account.verified"
CONTRACT_ACCEPTED = "merchant.contract.accepted"
FEE_TIER_CHANGED = "merchant.fee_tier.changed"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
