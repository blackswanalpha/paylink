from __future__ import annotations

import uuid
from decimal import Decimal

from app.events import publisher as ev
from app.events.consumer import (
    PAYLINK_REQUESTED,
    PAYMENT_INITIATED,
    ComplianceEventConsumer,
)
from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


def test_logical_event_names() -> None:
    # Pinned by identity-service's KycConsumer + paylink-service — these strings must not drift.
    assert ev.KYC_PASSED == "compliance.kyc.passed"
    assert ev.KYC_FAILED == "compliance.kyc.failed"
    assert ev.CHECK_PASSED == "compliance.check.passed"
    assert ev.CHECK_FAILED == "compliance.check.failed"
    assert ev.FLAG_RAISED == "compliance.flag.raised"


async def test_log_publisher_runs() -> None:
    await LogPublisher().publish(ev.KYC_PASSED, {"user_id": "x", "tier": 2})


async def test_noop_publisher_silent() -> None:
    assert await NoopPublisher().publish("x", {}) is None


def test_build_publisher_modes() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)


class _FakeRisk:
    def __init__(self) -> None:
        self.calls: list[tuple[uuid.UUID, str, Decimal | None, str | None]] = []

    async def record_activity(
        self,
        *,
        user_id: uuid.UUID,
        action: str,
        amount: Decimal | None,
        currency: str | None,
    ) -> None:
        self.calls.append((user_id, action, amount, currency))


async def test_consumer_records_payment_initiated_activity() -> None:
    risk = _FakeRisk()
    consumer = ComplianceEventConsumer(risk)  # type: ignore[arg-type]
    uid = str(uuid.uuid4())
    await consumer.handle(
        PAYMENT_INITIATED, {"user_id": uid, "amount": "150.50", "currency": "KES"}
    )
    assert risk.calls == [(uuid.UUID(uid), PAYMENT_INITIATED, Decimal("150.50"), "KES")]


async def test_consumer_payment_initiated_without_amount() -> None:
    risk = _FakeRisk()
    consumer = ComplianceEventConsumer(risk)  # type: ignore[arg-type]
    uid = str(uuid.uuid4())
    await consumer.handle(PAYMENT_INITIATED, {"user_id": uid})
    assert risk.calls == [(uuid.UUID(uid), PAYMENT_INITIATED, None, None)]


async def test_consumer_paylink_requested_is_noop() -> None:
    risk = _FakeRisk()
    consumer = ComplianceEventConsumer(risk)  # type: ignore[arg-type]
    await consumer.handle(PAYLINK_REQUESTED, {"user_id": str(uuid.uuid4())})
    assert risk.calls == []


async def test_consumer_ignores_unknown_and_bad() -> None:
    risk = _FakeRisk()
    consumer = ComplianceEventConsumer(risk)  # type: ignore[arg-type]
    await consumer.handle("compliance.unknown", {"user_id": str(uuid.uuid4())})  # unknown → no-op
    await consumer.handle(PAYMENT_INITIATED, {})  # missing user_id → no-op
    await consumer.handle(PAYMENT_INITIATED, {"user_id": "not-a-uuid"})  # bad uuid → no-op
    await consumer.handle(PAYMENT_INITIATED, {"user_id": str(uuid.uuid4()), "amount": "junk"})
    # The last call records activity with amount=None (junk amount tolerated).
    assert len(risk.calls) == 1
    assert risk.calls[0][2] is None
