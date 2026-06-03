"""Monthly invoice sweeper — generates the current period; resilient loop."""

from __future__ import annotations

import asyncio
import uuid
from datetime import UTC, datetime
from decimal import Decimal
from types import SimpleNamespace
from typing import Any

import fakeredis.aioredis
import pytest

import app.sweeper.run as sweeper_run
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from tests._support import FakeFxProvider, FakePricingRepository, make_settings


class _DummySession:
    async def commit(self) -> None: ...

    async def __aenter__(self) -> _DummySession:
        return self

    async def __aexit__(self, *_a: Any) -> bool:
        return False


def _app(repo: FakePricingRepository) -> SimpleNamespace:
    state = SimpleNamespace(
        settings=make_settings(),
        publisher=NoopPublisher(),
        fx_provider=FakeFxProvider(),
        redis=fakeredis.aioredis.FakeRedis(decode_responses=True),
        ledger_poster=NoopLedgerPoster(),
        sessionmaker=lambda: _DummySession(),
    )
    return SimpleNamespace(state=state)


async def test_run_once_generates_current_period(monkeypatch: pytest.MonkeyPatch) -> None:
    repo = FakePricingRepository()
    period = datetime.now(UTC).strftime("%Y-%m")
    await repo.insert_accrual(
        merchant_id=uuid.uuid4(),
        period=period,
        amount=Decimal(100),
        currency="KES",
        source_ref="s1",
        occurred_at=datetime.now(UTC),
    )
    monkeypatch.setattr(sweeper_run, "PricingRepository", lambda session: repo)
    assert await sweeper_run._run_once(_app(repo)) == 1


async def test_run_swallows_error_then_cancels(monkeypatch: pytest.MonkeyPatch) -> None:
    seen = {"n": 0}

    async def boom(_app: Any) -> int:
        seen["n"] += 1
        raise RuntimeError("transient")

    async def fake_sleep(_s: float) -> None:
        raise asyncio.CancelledError()

    monkeypatch.setattr(sweeper_run, "_run_once", boom)
    monkeypatch.setattr(sweeper_run.asyncio, "sleep", fake_sleep)
    with pytest.raises(asyncio.CancelledError):
        await sweeper_run.run(_app(FakePricingRepository()))
    assert seen["n"] == 1
