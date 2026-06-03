"""Ledger seam (A.6) — no-op default; called by generation only when the flag is on."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal

from app.domain.invoicing_service import InvoicingService
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from tests._support import FakePricingRepository, make_settings, noop_commit


class SpyPoster:
    def __init__(self) -> None:
        self.calls = 0

    async def post_platform_fee(
        self,
        *,
        invoice_id: uuid.UUID,
        merchant_id: uuid.UUID,
        period: str,
        currency: str,
        total_fee: Decimal,
    ) -> None:
        self.calls += 1


async def test_noop_poster_returns_without_io() -> None:
    await NoopLedgerPoster().post_platform_fee(
        invoice_id=uuid.uuid4(),
        merchant_id=uuid.uuid4(),
        period="2026-05",
        currency="KES",
        total_fee=Decimal(100),
    )


async def _seed_accrual(repo: FakePricingRepository, mid: uuid.UUID) -> None:
    await repo.insert_accrual(
        merchant_id=mid,
        period="2026-05",
        amount=Decimal(100),
        currency="KES",
        source_ref="s1",
        occurred_at=datetime(2026, 5, 1, tzinfo=UTC),
    )


async def test_poster_called_when_enabled() -> None:
    repo, spy = FakePricingRepository(), SpyPoster()
    mid = uuid.uuid4()
    await _seed_accrual(repo, mid)
    svc = InvoicingService(
        repo, noop_commit, NoopPublisher(), spy, make_settings(ledger_posting_enabled=True)
    )
    await svc.generate_for_period("2026-05")
    assert spy.calls == 1


async def test_poster_not_called_when_disabled() -> None:
    repo, spy = FakePricingRepository(), SpyPoster()
    mid = uuid.uuid4()
    await _seed_accrual(repo, mid)
    svc = InvoicingService(
        repo, noop_commit, NoopPublisher(), spy, make_settings(ledger_posting_enabled=False)
    )
    await svc.generate_for_period("2026-05")
    assert spy.calls == 0
