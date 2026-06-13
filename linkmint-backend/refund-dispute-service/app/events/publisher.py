"""Domain-event publisher seam.

Events are referenced by their **logical name** (catalog.md). The durable record of every event is
the ``refund.refund_events`` outbox table, written in-transaction by the service; the work15 relay
drains it onto Kafka. The bus derives the topic from the name's first dot-segment (``topic_for``),
so ``refund.*`` route to the ``refund`` topic and ``dispute.*`` to the ``dispute`` topic (both
created by ``redpanda-init``). The inline :class:`Publisher` is an in-process echo (log/noop) used
until/unless the relay runs.

INVARIANT: payloads carry ids/amount metadata only — never secrets or PII (catalog.md).
Non-custodial
(A.1): ``refund.reversal.instructed`` is an INSTRUCTION to a rail, not a fund movement.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# refund.* — topic "refund"
REFUND_REQUESTED = "refund.requested"
REFUND_APPROVED = "refund.approved"
REFUND_REJECTED = "refund.rejected"
REFUND_REVERSAL_INSTRUCTED = "refund.reversal.instructed"  # the non-custodial instruction (A.1)
REFUND_PROCESSING = "refund.processing"
REFUND_COMPLETED = "refund.completed"
REFUND_FAILED = "refund.failed"
REFUND_CLAWBACK_REQUESTED = "refund.clawback.requested"  # the work23 settlement contract

# dispute.* — topic "dispute"
DISPUTE_OPENED = "dispute.opened"
DISPUTE_EVIDENCE_ADDED = "dispute.evidence_added"
DISPUTE_SUBMITTED = "dispute.submitted"
DISPUTE_WON = "dispute.won"
DISPUTE_LOST = "dispute.lost"
DISPUTE_EXPIRED = "dispute.expired"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
