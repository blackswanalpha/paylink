"""chain.paylink.verified consumer → mark the backed invoice PAID (idempotent)."""

from __future__ import annotations

import uuid

from app.domain.services import ServiceDeps, build_services
from app.events.consumer import InvoiceEventConsumer
from app.events.stub import NoopPublisher
from tests._support import FakeInvoiceRepository, FakePaylink, make_settings, noop_commit


def _consumer(repo: FakeInvoiceRepository) -> InvoiceEventConsumer:
    deps = ServiceDeps(
        repo=repo,  # type: ignore[arg-type]
        commit=noop_commit,
        settings=make_settings(),
        publisher=NoopPublisher(),
        paylink=FakePaylink(),
    )
    return InvoiceEventConsumer(build_services(deps).invoices)


async def test_verified_marks_paid() -> None:
    repo = FakeInvoiceRepository()
    row = repo.seed(merchant_id=uuid.uuid4(), status="OPEN", pl_id="PLK_x")
    await _consumer(repo).handle("chain.paylink.verified", {"entity_id": "PLK_x"})
    assert row.status == "PAID"
    assert row.paid_at is not None
    assert "invoice.paid" in [k for (_i, k, _p) in repo.events]


async def test_verified_is_idempotent() -> None:
    repo = FakeInvoiceRepository()
    repo.seed(merchant_id=uuid.uuid4(), status="OPEN", pl_id="PLK_y")
    consumer = _consumer(repo)
    await consumer.handle("chain.paylink.verified", {"entity_id": "PLK_y"})
    await consumer.handle("chain.paylink.verified", {"entity_id": "PLK_y"})
    paid = [k for (_i, k, _p) in repo.events if k == "invoice.paid"]
    assert len(paid) == 1  # second delivery is a no-op


async def test_verified_marks_overdue_invoice_paid() -> None:
    repo = FakeInvoiceRepository()
    row = repo.seed(merchant_id=uuid.uuid4(), status="OVERDUE", pl_id="PLK_z")
    await _consumer(repo).handle("chain.paylink.verified", {"entity_id": "PLK_z"})
    assert row.status == "PAID"


async def test_unknown_event_is_noop() -> None:
    repo = FakeInvoiceRepository()
    await _consumer(repo).handle("payment.initiated", {"x": 1})
    assert repo.events == []


async def test_missing_plid_is_noop() -> None:
    repo = FakeInvoiceRepository()
    await _consumer(repo).handle("chain.paylink.verified", {})
    assert repo.events == []


async def test_unknown_plid_is_noop() -> None:
    repo = FakeInvoiceRepository()
    await _consumer(repo).handle("chain.paylink.verified", {"entity_id": "PLK_missing"})
    assert repo.events == []
