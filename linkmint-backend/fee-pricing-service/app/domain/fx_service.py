"""FX rate orchestration: Redis hot cache → provider → fallback, with durable audit + event.

``rate_for`` is the lock path the quote endpoint calls: a cache hit returns immediately (no side
effects); a miss fetches from the provider (or the configured fallback table), caches it for
``fx_cache_ttl_seconds``, writes a durable ``fx_rates`` audit row, emits ``fx.rate.updated`` via the
outbox, and commits. The returned :class:`Rate` is what the quote then LOCKS onto its row.
"""

from __future__ import annotations

import json
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any, Protocol

from app.config import Settings
from app.db.models import FxRateRow
from app.db.repositories import PricingRepository
from app.errors import AppError, ErrorCode
from app.events.publisher import FX_RATE_UPDATED, Publisher
from app.fx.provider import FxProvider, Rate
from app.fx.static import parse_rate_table
from app.logging import get_logger

log = get_logger("pricing.fx")

_Commit = Callable[[], Awaitable[None]]


class RedisLike(Protocol):
    async def get(self, key: str) -> Any: ...
    async def set(self, key: str, value: str, *, ex: int | None = None) -> Any: ...


class FxService:
    def __init__(
        self,
        repo: PricingRepository,
        commit: _Commit,
        publisher: Publisher,
        provider: FxProvider,
        redis: RedisLike,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._provider = provider
        self._redis = redis
        self._settings = settings
        self._fallback = parse_rate_table(settings.fx_fallback_rates)

    def _cache_key(self, base: str, quote: str) -> str:
        return f"{self._settings.service_name}:fx:{base}:{quote}"

    @staticmethod
    def _encode(r: Rate) -> str:
        return json.dumps(
            {
                "base": r.base,
                "quote": r.quote,
                "rate": str(r.rate),
                "source": r.source,
                "fetched_at": r.fetched_at.isoformat(),
            }
        )

    @staticmethod
    def _decode(raw: str) -> Rate:
        d = json.loads(raw)
        return Rate(
            base=d["base"],
            quote=d["quote"],
            rate=Decimal(str(d["rate"])),
            source=d["source"],
            fetched_at=datetime.fromisoformat(d["fetched_at"]),
        )

    async def rate_for(self, base: str, quote: str) -> Rate:
        """Resolve (and lock) the mid-market rate for ``base→quote``. Identity pairs return 1."""
        base, quote = base.upper(), quote.upper()
        if base == quote:
            return Rate(base, quote, Decimal(1), "identity", datetime.now(UTC))

        key = self._cache_key(base, quote)
        cached = await self._redis.get(key)
        if cached:
            try:
                return self._decode(cached)
            except (ValueError, KeyError):
                log.warning("fx_cache_decode_failed", key=key)  # treat as a miss

        rate = await self._provider.get_rate(base, quote)
        if rate is None:
            fb = self._fallback.get((base, quote))
            if fb is not None:
                rate = Rate(base, quote, fb, "fallback", datetime.now(UTC))
        if rate is None:
            raise AppError(
                ErrorCode.FX_UNAVAILABLE,
                f"no FX rate for {base}->{quote}",
                details={"base": base, "quote": quote},
            )

        await self._redis.set(key, self._encode(rate), ex=self._settings.fx_cache_ttl_seconds)
        await self._repo.insert_fx_rate(
            FxRateRow(
                base_currency=base,
                quote_currency=quote,
                rate=rate.rate,
                source=rate.source,
            )
        )
        payload = {
            "base": base,
            "quote": quote,
            "rate": str(rate.rate),
            "source": rate.source,
            "fetched_at": rate.fetched_at.isoformat(),
        }
        await self._repo.add_event(f"{base}:{quote}", FX_RATE_UPDATED, payload)
        await self._publisher.publish(FX_RATE_UPDATED, payload)
        await self._commit()
        log.info("fx_rate_locked", base=base, quote=quote, source=rate.source)
        return rate
