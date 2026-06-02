"""Async client for paylink-service (work01) — mints the aggregated PayLink on finalize.

A finalized invoice aggregates its line totals into a single PayLink: the merchant's ``payee_addr``
is the PayLink ``receiver`` (and is forwarded as ``X-Creator-Addr`` so the merchant is also the
PayLink creator/owner, matching paylink-service's ``caller_address`` seam). Transport or non-201
responses raise :class:`PaylinkError`; the domain maps that to a 502 PAYLINK_UNAVAILABLE. The client
never moves funds (non-custodial invariant A.1).
"""

from __future__ import annotations

from datetime import datetime
from typing import Any

import httpx

from app.logging import get_logger

log = get_logger("invoice.paylinks")


class PaylinkError(Exception):
    """paylink-service could not be reached or returned an unusable response."""


class PaylinkClient:
    def __init__(
        self,
        base_url: str,
        http: httpx.AsyncClient,
        *,
        internal_token: str | None = None,
        timeout: float = 5.0,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._http = http
        self._token = internal_token
        self._timeout = timeout

    async def create(
        self,
        *,
        receiver: str,
        amount: int,
        currency: str,
        expiry: datetime,
        usage: str = "single",
        metadata: dict[str, Any] | None = None,
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        """Create a PayLink. Raises :class:`PaylinkError` on any transport / protocol failure.

        ``idempotency_key`` is forwarded to paylink-service so a retried finalize re-uses the same
        PayLink instead of minting a second one (end-to-end idempotent mint).
        """
        body: dict[str, Any] = {
            "receiver": receiver,
            "amount": amount,
            "currency": currency,
            "expiry": expiry.isoformat(),
            "usage": usage,
        }
        if metadata:
            body["metadata"] = metadata

        headers = {"Content-Type": "application/json", "X-Creator-Addr": receiver}
        if self._token:
            headers["X-Internal-Token"] = self._token
        if idempotency_key:
            headers["Idempotency-Key"] = idempotency_key

        try:
            resp = await self._http.post(
                f"{self._base}/v1/paylinks",
                json=body,
                headers=headers,
                timeout=self._timeout,
            )
        except httpx.HTTPError as exc:
            raise PaylinkError(f"paylinks create unreachable: {exc}") from exc
        if resp.status_code != 201:
            raise PaylinkError(f"paylinks create returned http {resp.status_code}")
        try:
            data: dict[str, Any] = resp.json()
        except ValueError as exc:
            raise PaylinkError(f"paylinks create bad response: {exc}") from exc
        return data
