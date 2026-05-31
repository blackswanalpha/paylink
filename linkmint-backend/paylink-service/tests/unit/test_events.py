from __future__ import annotations

from app.config import Settings
from app.events.publisher import PAYLINK_CREATED
from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


async def test_log_publisher_does_not_collide_with_structlog_event() -> None:
    # Regression: structlog binds the positional message to `event`; the publisher must not also
    # pass an `event=` kwarg (that raised TypeError and 500'd create).
    await LogPublisher().publish(PAYLINK_CREATED, {"pl_id": "0x1", "chain_tx_hash": "0xabc"})


async def test_noop_publisher() -> None:
    await NoopPublisher().publish(PAYLINK_CREATED, {"pl_id": "0x1"})


def test_build_publisher_selects_mode() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)


def test_settings_accepts_log_mode() -> None:
    assert Settings(event_publisher_mode="log").event_publisher_mode == "log"
