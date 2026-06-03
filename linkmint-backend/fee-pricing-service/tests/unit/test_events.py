"""Publisher seam — logical names, build_publisher modes, in-process publish."""

from __future__ import annotations

from app.events.publisher import (
    FX_RATE_UPDATED,
    INVOICE_PLATFORM_FEE_ISSUED,
    PRICING_FEE_QUOTE_ISSUED,
)
from app.events.stub import LogPublisher, NoopPublisher, build_publisher
from tests._support import make_settings


def test_logical_names() -> None:
    assert PRICING_FEE_QUOTE_ISSUED == "pricing.fee_quote.issued"
    assert FX_RATE_UPDATED == "fx.rate.updated"
    assert INVOICE_PLATFORM_FEE_ISSUED == "invoice.platform_fee.issued"


def test_build_publisher_modes() -> None:
    assert isinstance(build_publisher(make_settings(event_publisher_mode="log")), LogPublisher)
    assert isinstance(build_publisher(make_settings(event_publisher_mode="noop")), NoopPublisher)
    # kafka mode → the inline seam goes quiet (the outbox relay is the real producer).
    assert isinstance(build_publisher(make_settings(event_publisher_mode="kafka")), NoopPublisher)


async def test_publishers_accept_metadata_payload() -> None:
    payload = {"quote_id": "q1", "merchant_id": "m1", "platform_fee": 250}
    await LogPublisher().publish(PRICING_FEE_QUOTE_ISSUED, payload)
    await NoopPublisher().publish(PRICING_FEE_QUOTE_ISSUED, payload)
