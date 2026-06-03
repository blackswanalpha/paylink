"""POST /v1/internal/invoices/platform-fee/run — monthly aggregation + idempotency."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import FakePricingRepository


def _accrue(client: TestClient, merchant_id: str, amount: int, ref: str) -> None:
    r = client.post(
        "/v1/internal/accruals",
        json={
            "merchant_id": merchant_id,
            "amount": amount,
            "currency": "KES",
            "source_ref": ref,
            "occurred_at": "2026-05-15T12:00:00+00:00",
        },
    )
    assert r.status_code == 202


def test_run_aggregates_and_is_idempotent(
    client: TestClient, fake_repo: FakePricingRepository
) -> None:
    mid = str(uuid.uuid4())
    _accrue(client, mid, 2_500, "p1")
    _accrue(client, mid, 1_500, "p2")

    r = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert r.status_code == 200
    gen = r.json()["generated"]
    assert len(gen) == 1
    assert gen[0]["total_fee"] == 4_000
    assert gen[0]["line_count"] == 2
    assert "invoice.platform_fee.issued" in fake_repo.event_kinds()
    assert all(a.invoice_id is not None for a in fake_repo.accruals)  # stamped

    # Clean re-run is a no-op: every accrual is already billed → nothing to generate or skip.
    r2 = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert r2.json()["generated"] == []
    assert len(fake_repo.invoices) == 1

    # A NEW accrual for the same merchant+period after the invoice exists → skipped (stays unbilled).
    _accrue(client, mid, 700, "p3")
    r3 = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert r3.json()["generated"] == []
    assert r3.json()["skipped_existing"] == 1
    assert len(fake_repo.invoices) == 1
    assert any(a.source_ref == "p3" and a.invoice_id is None for a in fake_repo.accruals)


def test_run_fans_out_per_merchant(client: TestClient, fake_repo: FakePricingRepository) -> None:
    m1, m2 = str(uuid.uuid4()), str(uuid.uuid4())
    _accrue(client, m1, 1_000, "a1")
    _accrue(client, m2, 2_000, "b1")
    r = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert len(r.json()["generated"]) == 2
    assert len(fake_repo.invoices) == 2


def test_run_invalid_period(client: TestClient) -> None:
    r = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-5"})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PERIOD"


def test_run_empty_period_generates_nothing(client: TestClient) -> None:
    r = client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2099-01"})
    assert r.status_code == 200
    assert r.json()["generated"] == []
