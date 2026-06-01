"""The retry backoff schedule (spec §2.7: 30s, 2m, 10m, 1h, 6h; max 5)."""

from __future__ import annotations

from app.delivery.backoff import BACKOFF_SCHEDULE_SECONDS, MAX_RETRIES, next_countdown


def test_schedule_is_the_spec_values() -> None:
    assert BACKOFF_SCHEDULE_SECONDS == (30, 120, 600, 3600, 21600)
    assert MAX_RETRIES == 5


def test_next_countdown_walks_the_schedule() -> None:
    assert [next_countdown(n) for n in range(1, 6)] == [30, 120, 600, 3600, 21600]


def test_next_countdown_exhausted_returns_none() -> None:
    assert next_countdown(6) is None
    assert next_countdown(99) is None


def test_next_countdown_zero_or_negative_is_none() -> None:
    assert next_countdown(0) is None
    assert next_countdown(-1) is None
