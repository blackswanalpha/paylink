"""The pure DeliveryRunner — full transition matrix + exact backoff countdowns (no Celery/DB)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

from app.delivery.runner import NOOP, DeliveryRunner
from app.domain.models import EXHAUSTED, FAILED, SENT
from tests._support import FakeChannels, FakeDeliveryStore, FakeProvider, make_record

FIXED = datetime(2026, 6, 1, 12, 0, 0, tzinfo=UTC)


def _runner(store: FakeDeliveryStore, provider: FakeProvider | None) -> DeliveryRunner:
    return DeliveryRunner(store, FakeChannels(provider), clock=lambda: FIXED)


def test_success_marks_sent() -> None:
    rec = make_record(channel="sms")
    store = FakeDeliveryStore({rec.delivery_id: rec})
    runner = _runner(store, FakeProvider(fail_times=0))

    outcome = runner.run_once(rec.delivery_id)

    assert outcome.status == SENT
    assert outcome.should_retry is False
    assert store.sent and store.sent[0][0] == rec.delivery_id


def test_first_failure_schedules_30s_retry() -> None:
    rec = make_record(channel="sms", attempts=0)
    store = FakeDeliveryStore({rec.delivery_id: rec})
    runner = _runner(store, FakeProvider(fail_times=1))

    outcome = runner.run_once(rec.delivery_id)

    assert outcome.status == FAILED
    assert outcome.should_retry is True
    assert outcome.countdown == 30
    delivery_id, attempts, _err, next_retry_at = store.failed[0]
    assert attempts == 1
    assert next_retry_at == FIXED + timedelta(seconds=30)


def test_full_backoff_then_exhaustion() -> None:
    rec = make_record(channel="sms", attempts=0)
    store = FakeDeliveryStore({rec.delivery_id: rec})
    runner = _runner(store, FakeProvider(fail_times=100))  # always fails

    outcomes = [runner.run_once(rec.delivery_id) for _ in range(6)]

    assert [o.countdown for o in outcomes[:5]] == [30, 120, 600, 3600, 21600]
    assert all(o.status == FAILED for o in outcomes[:5])
    assert outcomes[5].status == EXHAUSTED
    assert outcomes[5].should_retry is False
    assert store.exhausted and store.exhausted[0][1] == 6  # attempts at exhaustion


def test_noop_on_terminal_row() -> None:
    rec = make_record(status=SENT)
    store = FakeDeliveryStore({rec.delivery_id: rec})
    runner = _runner(store, FakeProvider())
    assert runner.run_once(rec.delivery_id).status == NOOP


def test_noop_on_missing_row() -> None:
    rec = make_record()
    store = FakeDeliveryStore({})  # not present
    runner = _runner(store, FakeProvider())
    assert runner.run_once(rec.delivery_id).status == NOOP


def test_unknown_channel_exhausts() -> None:
    rec = make_record(channel="carrier-pigeon")
    store = FakeDeliveryStore({rec.delivery_id: rec})
    runner = DeliveryRunner(store, FakeChannels(provider=None), clock=lambda: FIXED)
    outcome = runner.run_once(rec.delivery_id)
    assert outcome.status == EXHAUSTED
    assert store.exhausted[0][0] == rec.delivery_id
