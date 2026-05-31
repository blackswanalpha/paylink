from __future__ import annotations

import uuid

from app.events import publisher as ev
from app.events.consumer import KYC_FAILED, KYC_PASSED, KycConsumer
from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


async def test_log_publisher_runs() -> None:
    await LogPublisher().publish(ev.USER_REGISTERED, {"user_id": "x"})


async def test_noop_publisher_silent() -> None:
    assert await NoopPublisher().publish("x", {}) is None


def test_build_publisher_modes() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)


class _FakeUsers:
    def __init__(self) -> None:
        self.calls: list[tuple[uuid.UUID, int]] = []

    async def set_kyc_tier(self, user_id: uuid.UUID, tier: int) -> None:
        self.calls.append((user_id, tier))


async def test_kyc_consumer_routes() -> None:
    users = _FakeUsers()
    consumer = KycConsumer(users)  # type: ignore[arg-type]
    uid = str(uuid.uuid4())
    await consumer.handle(KYC_PASSED, {"user_id": uid, "tier": 2})
    await consumer.handle(KYC_FAILED, {"user_id": uid})
    await consumer.handle("compliance.kyc.unknown", {"user_id": uid})  # ignored
    await consumer.handle(KYC_PASSED, {})  # missing user_id → no-op
    assert (uuid.UUID(uid), 2) in users.calls
    assert (uuid.UUID(uid), 0) in users.calls
    assert len(users.calls) == 2
