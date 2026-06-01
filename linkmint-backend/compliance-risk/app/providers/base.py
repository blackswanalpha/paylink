"""KYC provider protocol + result types + the upstream-error type.

A ``KycProvider`` starts a verification session and parses the vendor's async callback into a
normalized :class:`CallbackResult`. The in-process :class:`~app.providers.fake.FakeKycProvider`
(``stub``) is the MVP default AND the test double; the httpx ``HttpKycProvider``
is the production drop-in (Jumio/Smile/Onfido). Both normalize to the same shapes, so
``KycService`` never knows which vendor answered.

INVARIANT: ``CallbackResult.metadata`` may contain vendor PII — it is passed through
:func:`app.redaction.redact` before any write/log/emit. Raw PII never crosses this boundary into
persistence.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from typing import Any, Protocol


class UpstreamError(Exception):
    """A KYC provider upstream call failed (network error or non-2xx/404 response)."""

    def __init__(self, provider: str, *, status: int | None = None) -> None:
        self.provider = provider
        self.status = status
        super().__init__(f"{provider} KYC upstream returned an error (status={status})")


@dataclass(frozen=True)
class StartResult:
    """The vendor's hosted-verification handle returned to the caller."""

    session_id: str
    provider_url: str


@dataclass(frozen=True)
class CallbackResult:
    """A vendor callback normalized for ``KycService.apply_callback``.

    ``metadata`` is raw vendor metadata (possibly PII) — REDACTED before any persistence/log/emit.
    """

    user_id: uuid.UUID
    passed: bool
    tier_granted: int
    provider_ref: str
    metadata: dict[str, Any] = field(default_factory=dict)


class KycProvider(Protocol):
    name: str

    async def start(self, user_id: uuid.UUID, tier: int) -> StartResult: ...

    def parse_callback(self, body: dict[str, Any]) -> CallbackResult: ...
