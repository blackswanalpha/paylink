"""Overdue sweep — flip OPEN past due → OVERDUE and emit invoice.overdue."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

from app.domain.services import InvoiceService, ServiceDeps, build_services
from app.events.stub import NoopPublisher
from tests._support import FakeInvoiceRepository, FakePaylink, make_settings, noop_commit


def _svc(repo: FakeInvoiceRepository) -> InvoiceService:
    deps = ServiceDeps(
        repo=repo,  # type: ignore[arg-type]
        commit=noop_commit,
        settings=make_settings(),
        publisher=NoopPublisher(),
        paylink=FakePaylink(),
    )
    return build_services(deps).invoices


async def test_sweep_marks_only_past_due() -> None:
    repo = FakeInvoiceRepository()
    mid = uuid.uuid4()
    overdue = repo.seed(
        merchant_id=mid, status="OPEN", due_at=datetime.now(UTC) - timedelta(days=1)
    )
    fresh = repo.seed(merchant_id=mid, status="OPEN", due_at=datetime.now(UTC) + timedelta(days=1))
    n = await _svc(repo).sweep_overdue()
    assert n == 1
    assert overdue.status == "OVERDUE"
    assert fresh.status == "OPEN"
    assert "invoice.overdue" in [k for (_i, k, _p) in repo.events]


async def test_sweep_noop_when_none_due() -> None:
    repo = FakeInvoiceRepository()
    assert await _svc(repo).sweep_overdue() == 0
    assert repo.events == []
