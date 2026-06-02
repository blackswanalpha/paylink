"""The (forward-symmetry) event publisher seam."""

from __future__ import annotations

from app.events.publisher import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


async def test_log_publisher_does_not_raise() -> None:
    await LogPublisher().publish("notification.delivered", {"delivery_id": "d1"})


async def test_noop_publisher_returns_none() -> None:
    assert await NoopPublisher().publish("notification.delivered", {}) is None


def test_build_publisher_selects_by_mode() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)
