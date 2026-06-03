"""HTTP FX provider seam — a live mid-market rate source.

Disabled by default (``PRICING_FX_PROVIDER=static``). When enabled it GETs
``{base_url}?base=<base>&quote=<quote>`` and expects ``{"rate": "<decimal>"}``. Network/parse
failures return None so the caller falls back to the configured fallback table (never a hard
failure on the quote path). The outbound client is the shared traced httpx client (work18).
"""

from __future__ import annotations

from datetime import UTC, datetime
from decimal import Decimal, InvalidOperation
from urllib.parse import urlparse

import httpx

from app.fx.provider import Rate
from app.logging import get_logger

log = get_logger("pricing.fx")


class HttpFxProvider:
    def __init__(self, client: httpx.AsyncClient, base_url: str, *, timeout: float = 5.0) -> None:
        self._client = client
        self._base_url = base_url
        self._timeout = timeout
        self._source = f"http:{urlparse(base_url).hostname or 'provider'}"

    async def get_rate(self, base: str, quote: str) -> Rate | None:
        base, quote = base.upper(), quote.upper()
        if base == quote:
            return Rate(base, quote, Decimal(1), "identity", datetime.now(UTC))
        try:
            resp = await self._client.get(
                self._base_url, params={"base": base, "quote": quote}, timeout=self._timeout
            )
            resp.raise_for_status()
            rate = Decimal(str(resp.json()["rate"]))
        except (httpx.HTTPError, KeyError, ValueError, InvalidOperation) as exc:
            log.warning("fx_http_fetch_failed", base=base, quote=quote, error=str(exc))
            return None
        return Rate(base, quote, rate, self._source, datetime.now(UTC))
