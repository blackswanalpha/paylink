"""MerchantPricingEventConsumer — keeps merchant_pricing in sync; tolerant + idempotent."""

from __future__ import annotations

import uuid

import fakeredis.aioredis

from app.domain.services import ServiceDeps, build_services
from app.events.consumer import MerchantPricingEventConsumer
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from tests._support import FakeFxProvider, FakePricingRepository, make_settings, noop_commit


def _consumer(repo: FakePricingRepository) -> MerchantPricingEventConsumer:
    deps = ServiceDeps(
        repo=repo,  # type: ignore[arg-type]
        commit=noop_commit,
        settings=make_settings(),
        publisher=NoopPublisher(),
        fx_provider=FakeFxProvider(),
        redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
        ledger=NoopLedgerPoster(),
    )
    return MerchantPricingEventConsumer(build_services(deps).pricing)


async def test_onboarded_seeds_default_tier_and_org() -> None:
    repo = FakePricingRepository()
    mid, org = uuid.uuid4(), uuid.uuid4()
    await _consumer(repo).handle(
        "merchant.onboarded", {"merchant_id": str(mid), "org_id": str(org)}
    )
    row = repo.merchant_pricing[mid]
    assert row.tier == "standard"
    assert row.org_id == org
    assert row.source == "onboarded"


async def test_fee_tier_changed_updates_tier_keeps_org() -> None:
    repo = FakePricingRepository()
    mid, org = uuid.uuid4(), uuid.uuid4()
    consumer = _consumer(repo)
    await consumer.handle("merchant.onboarded", {"merchant_id": str(mid), "org_id": str(org)})
    await consumer.handle(
        "merchant.fee_tier.changed", {"merchant_id": str(mid), "tier": "enterprise"}
    )
    row = repo.merchant_pricing[mid]
    assert row.tier == "enterprise"
    assert row.org_id == org  # not cleared by the tier-change event


async def test_unknown_tier_falls_back_to_default() -> None:
    repo = FakePricingRepository()
    mid = uuid.uuid4()
    await _consumer(repo).handle(
        "merchant.fee_tier.changed", {"merchant_id": str(mid), "tier": "platinum"}
    )
    assert repo.merchant_pricing[mid].tier == "standard"  # tolerant — no raise


async def test_missing_merchant_id_is_noop() -> None:
    repo = FakePricingRepository()
    await _consumer(repo).handle("merchant.onboarded", {"org_id": str(uuid.uuid4())})
    assert repo.merchant_pricing == {}


async def test_redelivery_is_idempotent() -> None:
    repo = FakePricingRepository()
    mid, org = uuid.uuid4(), uuid.uuid4()
    consumer = _consumer(repo)
    payload = {"merchant_id": str(mid), "org_id": str(org)}
    await consumer.handle("merchant.onboarded", payload)
    await consumer.handle("merchant.onboarded", payload)
    assert len(repo.merchant_pricing) == 1


async def test_unknown_event_ignored() -> None:
    repo = FakePricingRepository()
    await _consumer(repo).handle("merchant.something.else", {"merchant_id": str(uuid.uuid4())})
    assert repo.merchant_pricing == {}
