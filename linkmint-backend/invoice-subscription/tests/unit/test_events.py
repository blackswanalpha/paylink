"""Publisher factory + in-process echo publishers."""

from __future__ import annotations

from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


def test_build_publisher_modes() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    # kafka mode silences the inline seam (the outbox relay is the real producer).
    assert isinstance(build_publisher(make_settings(event_publisher_mode="kafka")), NoopPublisher)


async def test_log_and_noop_publish_run() -> None:
    await LogPublisher().publish("invoice.created", {"invoice_id": "x"})
    await NoopPublisher().publish("invoice.created", {"invoice_id": "x"})
