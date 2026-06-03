"""FxService — cache → provider → fallback, with durable audit + lock-at-quote event."""

from __future__ import annotations

from decimal import Decimal

import fakeredis.aioredis
import pytest

from app.domain.fx_service import FxService
from app.errors import AppError, ErrorCode
from app.events.stub import NoopPublisher
from tests._support import FakeFxProvider, FakePricingRepository, make_settings, noop_commit


def _fx(
    provider: FakeFxProvider, repo: FakePricingRepository, redis: object, **over: object
) -> FxService:
    return FxService(repo, noop_commit, NoopPublisher(), provider, redis, make_settings(**over))  # type: ignore[arg-type]


def _redis() -> fakeredis.aioredis.FakeRedis:
    return fakeredis.aioredis.FakeRedis(decode_responses=True)


async def test_identity_pair_returns_one() -> None:
    repo, provider = FakePricingRepository(), FakeFxProvider()
    r = await _fx(provider, repo, _redis()).rate_for("KES", "KES")
    assert r.rate == Decimal(1)
    assert r.source == "identity"
    assert provider.calls == 0
    assert repo.fx_rates == []


async def test_cache_miss_fetches_caches_audits_emits() -> None:
    repo, provider, redis = FakePricingRepository(), FakeFxProvider(), _redis()
    r = await _fx(provider, repo, redis).rate_for("usd", "kes")
    assert r.rate == Decimal("129.50")
    assert provider.calls == 1
    assert len(repo.fx_rates) == 1
    assert "fx.rate.updated" in repo.event_kinds()
    assert await redis.get("fee-pricing-service:fx:USD:KES") is not None


async def test_cache_hit_does_not_refetch() -> None:
    repo, provider, redis = FakePricingRepository(), FakeFxProvider(), _redis()
    fx = _fx(provider, repo, redis)
    await fx.rate_for("USD", "KES")
    calls_after_first = provider.calls
    again = await fx.rate_for("USD", "KES")
    assert provider.calls == calls_after_first  # served from cache, no new fetch
    assert len(repo.fx_rates) == 1  # no new audit row
    assert again.rate == Decimal("129.50")


async def test_fallback_when_provider_has_no_rate() -> None:
    repo, provider, redis = FakePricingRepository(), FakeFxProvider(rates={}), _redis()
    r = await _fx(provider, repo, redis, fx_fallback_rates="GBP:KES=160.0").rate_for("GBP", "KES")
    assert r.source == "fallback"
    assert r.rate == Decimal("160.0")


async def test_unavailable_raises() -> None:
    repo, provider, redis = FakePricingRepository(), FakeFxProvider(rates={}), _redis()
    with pytest.raises(AppError) as exc:
        await _fx(provider, repo, redis, fx_fallback_rates="").rate_for("AUD", "KES")
    assert exc.value.code == ErrorCode.FX_UNAVAILABLE
