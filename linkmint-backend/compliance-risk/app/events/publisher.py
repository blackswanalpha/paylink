"""Domain-event publisher seam.

Events are referenced by their **logical name** (the backendfeatures.md §2.6 taxonomy). The concrete
Kafka/SQS transport (ADR-004) is delivered by **work15**; until then a publisher just logs or
no-ops. The durable record of every event is the ``compliance.compliance_events`` table, written
in-transaction by the service (so work15 can drain it as an outbox).

INVARIANT: payloads NEVER carry raw PII — only ids, tier, decision, and reason metadata.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by compliance-risk (backendfeatures.md §2.6).
KYC_PASSED = "compliance.kyc.passed"
KYC_FAILED = "compliance.kyc.failed"
CHECK_PASSED = "compliance.check.passed"
CHECK_FAILED = "compliance.check.failed"
FLAG_RAISED = "compliance.flag.raised"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
