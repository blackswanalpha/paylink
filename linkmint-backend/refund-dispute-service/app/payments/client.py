"""payment-orchestrator lookup client — refund eligibility (work02 seam).

A refund is eligible only for a payment that EXISTS and is SETTLED; the lookup also yields the
``paylink_id`` and the opaque ``rail`` (A.4) the reversal instruction targets. The orchestrator
stores no amounts (those live on-chain / on the PayLink), so the original amount is resolved
separately (verified_paylinks projection + paylink-service).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

import httpx

from app.errors import AppError, ErrorCode
from app.logging import get_logger

log = get_logger("refund.payments")


@dataclass(frozen=True)
class PaymentInfo:
    id: str
    paylink_id: str
    rail: str
    status: str  # INITIATED|SETTLED|CANCELLED|FAILED


class PaymentsClient(Protocol):
    async def get(self, payment_id: str) -> PaymentInfo | None:
        """Return the payment, or None when the orchestrator reports 404."""
        ...


class HttpPaymentsClient:
    def __init__(self, base_url: str, client: httpx.AsyncClient) -> None:
        self._base = base_url.rstrip("/")
        self._client = client

    async def get(self, payment_id: str) -> PaymentInfo | None:
        url = f"{self._base}/v1/payments/{payment_id}"
        try:
            resp = await self._client.get(url)
        except httpx.HTTPError as exc:
            log.warning("payment_lookup_error", payment_id=payment_id, error=str(exc))
            raise AppError(ErrorCode.UPSTREAM_UNAVAILABLE, "payment lookup failed") from exc
        if resp.status_code == 404:
            return None
        if resp.status_code != 200:
            log.warning("payment_lookup_status", payment_id=payment_id, status=resp.status_code)
            raise AppError(ErrorCode.UPSTREAM_UNAVAILABLE, "payment lookup failed")
        body = resp.json()
        return PaymentInfo(
            id=str(body.get("id", payment_id)),
            paylink_id=str(body.get("paylink_id", "")),
            rail=str(body.get("rail", "")),
            status=str(body.get("status", "")),
        )
