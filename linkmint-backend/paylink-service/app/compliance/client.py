"""Async client for the compliance-risk service (work12).

PayLink creation above ``amount_kyc_threshold`` is gated synchronously on
``POST /v1/risk/evaluate`` (internal, trusted-network — the gateway/mesh terminates mTLS; an
optional ``X-Internal-Token`` adds defense-in-depth, ADR-009). Transport or non-200 responses raise
:class:`ComplianceUnavailable`; the caller (``PayLinkService``) decides fail-open vs fail-closed.
This is the realized Flow E seam — the client never moves funds (A.1).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

import httpx

from app.logging import get_logger

log = get_logger("paylink.compliance")


class ComplianceUnavailable(Exception):
    """compliance-risk could not be reached or returned an unusable response."""


@dataclass(frozen=True)
class RiskDecision:
    """The decision returned by ``/v1/risk/evaluate`` (mirrors the wire shape exactly)."""

    decision: str  # allow | block | review
    score: float
    reasons: list[dict[str, Any]] = field(default_factory=list)


class ComplianceClient:
    def __init__(
        self,
        base_url: str,
        http: httpx.AsyncClient,
        *,
        internal_token: str | None = None,
        timeout: float = 3.0,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._http = http
        self._token = internal_token
        self._timeout = timeout

    async def evaluate(
        self,
        *,
        user_id: str,
        action: str,
        amount: int | None = None,
        currency: str | None = None,
        geo: str | None = None,
        registered_country: str | None = None,
        context: str | None = None,
    ) -> RiskDecision:
        """Ask compliance-risk for an allow/block/review decision. Raises
        :class:`ComplianceUnavailable` on any transport / protocol failure."""
        body: dict[str, Any] = {"user_id": user_id, "action": action}
        if amount is not None:
            body["amount"] = amount
        if currency is not None:
            body["currency"] = currency
        if geo is not None:
            body["geo"] = geo
        if registered_country is not None:
            body["registered_country"] = registered_country
        if context is not None:
            body["context"] = context

        headers = {"Content-Type": "application/json"}
        if self._token:
            headers["X-Internal-Token"] = self._token

        try:
            resp = await self._http.post(
                f"{self._base}/v1/risk/evaluate",
                json=body,
                headers=headers,
                timeout=self._timeout,
            )
        except httpx.HTTPError as exc:
            raise ComplianceUnavailable(f"risk/evaluate unreachable: {exc}") from exc
        if resp.status_code != 200:
            raise ComplianceUnavailable(f"risk/evaluate returned http {resp.status_code}")
        try:
            data = resp.json()
            return RiskDecision(
                decision=str(data["decision"]),
                score=float(data.get("score", 0.0)),
                reasons=list(data.get("reasons", [])),
            )
        except (ValueError, KeyError, TypeError) as exc:
            raise ComplianceUnavailable(f"risk/evaluate bad response: {exc}") from exc
