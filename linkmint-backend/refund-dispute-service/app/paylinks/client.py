"""paylink-service lookup client — original-amount fallback.

When the ``verified_paylinks`` projection hasn't yet seen the chain.paylink.verified event, the
original amount is resolved from paylink-service ``GET /v1/paylinks/{id}`` (which returns ``amount``
in integer minor units + ``currency``). Best-effort: any failure returns None so the caller applies
its amount-validation policy (strict → 502, lenient → accept caller amount).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

import httpx

from app.logging import get_logger

log = get_logger("refund.paylinks")


@dataclass(frozen=True)
class PaylinkAmount:
    amount_minor: int
    currency: str


class PaylinksClient(Protocol):
    async def get_amount(self, paylink_id: str) -> PaylinkAmount | None: ...


class HttpPaylinksClient:
    def __init__(self, base_url: str, client: httpx.AsyncClient) -> None:
        self._base = base_url.rstrip("/")
        self._client = client

    async def get_amount(self, paylink_id: str) -> PaylinkAmount | None:
        url = f"{self._base}/v1/paylinks/{paylink_id}"
        try:
            resp = await self._client.get(url)
        except httpx.HTTPError as exc:
            log.warning("paylink_lookup_error", paylink_id=paylink_id, error=str(exc))
            return None
        if resp.status_code != 200:
            return None
        body = resp.json()
        try:
            return PaylinkAmount(amount_minor=int(body["amount"]), currency=str(body["currency"]))
        except (KeyError, TypeError, ValueError):
            log.warning("paylink_amount_missing", paylink_id=paylink_id)
            return None
