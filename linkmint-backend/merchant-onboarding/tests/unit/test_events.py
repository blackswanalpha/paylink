from __future__ import annotations

import uuid

from app.domain.models import ReviewDecision
from app.events import publisher as ev
from app.events.consumer import (
    ADMIN_OVERRIDE_REINSTATE,
    ADMIN_OVERRIDE_SUSPEND,
    KYB_FAILED,
    KYB_PASSED,
    MerchantEventConsumer,
)
from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


async def test_log_publisher_runs() -> None:
    await LogPublisher().publish(ev.MERCHANT_ONBOARDED, {"merchant_id": "x"})


async def test_noop_publisher_silent() -> None:
    assert await NoopPublisher().publish("x", {}) is None


def test_build_publisher_modes() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)


class _FakeMerchants:
    def __init__(self) -> None:
        self.calls: list[tuple[uuid.UUID, ReviewDecision, str | None]] = []

    async def decide(
        self, *, merchant_id: uuid.UUID, decision: ReviewDecision, reason: str | None = None
    ) -> None:
        self.calls.append((merchant_id, decision, reason))


async def test_consumer_routes_all_known_events() -> None:
    merchants = _FakeMerchants()
    consumer = MerchantEventConsumer(merchants)  # type: ignore[arg-type]
    mid = str(uuid.uuid4())
    await consumer.handle(KYB_PASSED, {"merchant_id": mid})
    await consumer.handle(KYB_FAILED, {"merchant_id": mid, "reason": "sanctions"})
    await consumer.handle(ADMIN_OVERRIDE_SUSPEND, {"merchant_id": mid, "reason": "fraud"})
    await consumer.handle(ADMIN_OVERRIDE_REINSTATE, {"merchant_id": mid})
    decisions = [(d, r) for (_, d, r) in merchants.calls]
    assert (ReviewDecision.APPROVE, None) in decisions
    assert (ReviewDecision.REJECT, "sanctions") in decisions
    assert (ReviewDecision.SUSPEND, "fraud") in decisions
    assert (ReviewDecision.REINSTATE, None) in decisions
    assert len(merchants.calls) == 4


async def test_consumer_ignores_unknown_and_missing() -> None:
    merchants = _FakeMerchants()
    consumer = MerchantEventConsumer(merchants)  # type: ignore[arg-type]
    await consumer.handle("compliance.kyb.unknown", {"merchant_id": str(uuid.uuid4())})  # ignored
    await consumer.handle(KYB_PASSED, {})  # missing merchant_id → no-op
    await consumer.handle(KYB_PASSED, {"merchant_id": "not-a-uuid"})  # malformed id → no-op
    assert merchants.calls == []
