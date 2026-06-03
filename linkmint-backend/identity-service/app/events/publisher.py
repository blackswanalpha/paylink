"""Domain-event publisher seam.

Events are referenced by their **logical name** (the backendfeatures.md taxonomy). The concrete
Kafka/SQS transport (ADR-004) is delivered by **work15**; until then a publisher just logs or
no-ops. The durable record of every event is the ``identity.identity_events`` table, written
in-transaction by the service (so work15 can drain it as an outbox).

Payloads NEVER carry secrets — no passwords, full API keys, hashes, refresh tokens, or MFA secrets.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

# Logical event names produced by identity-service (backendfeatures.md §2.9).
USER_REGISTERED = "identity.user.registered"
USER_VERIFIED = "identity.user.verified"
USER_SUSPENDED = "identity.user.suspended"
ORG_CREATED = "identity.org.created"
MEMBER_ADDED = "identity.member.added"
MEMBER_REMOVED = "identity.member.removed"
API_KEY_ISSUED = "identity.api_key.issued"
API_KEY_REVOKED = "identity.api_key.revoked"
MFA_ENABLED = "identity.mfa.enabled"
AUTH_FAILED = "identity.auth.failed"
PASSWORD_RESET_REQUESTED = "identity.auth.password_reset_requested"
PASSWORD_RESET_SUCCEEDED = "identity.auth.password_reset_succeeded"


class Publisher(ABC):
    @abstractmethod
    async def publish(self, name: str, payload: dict[str, Any]) -> None: ...
