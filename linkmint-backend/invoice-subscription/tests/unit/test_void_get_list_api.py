"""void + read (GET one / list) endpoints."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

from fastapi.testclient import TestClient

from tests._support import FakeInvoiceRepository, create_body, merchant_headers


def _create(client: TestClient, uid: str) -> str:
    return client.post("/v1/invoices", json=create_body(), headers=merchant_headers(uid)).json()[
        "invoice_id"
    ]


def test_void_draft(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.post(f"/v1/invoices/{iid}/void", headers=merchant_headers(uid))
    assert r.status_code == 200
    assert r.json()["status"] == "VOID"
    assert "invoice.voided" in [k for (_i, k, _p) in fake_repo.events]


def test_void_already_void_409(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = uuid.uuid4()
    row = fake_repo.seed(merchant_id=uid, status="VOID")
    r = client.post(f"/v1/invoices/{row.invoice_id}/void", headers=merchant_headers(str(uid)))
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "INVALID_STATE"


def test_get_returns_invoice_with_lines(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.get(f"/v1/invoices/{iid}", headers=merchant_headers(uid))
    assert r.status_code == 200
    body = r.json()
    assert body["invoice_id"] == iid
    assert body["total"] == 3480
    assert len(body["lines"]) == 1
    assert body["lines"][0]["unit_price"] == 1500
    assert body["lines"][0]["quantity"] == "2"


def test_get_not_owned_404(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.get(f"/v1/invoices/{iid}", headers=merchant_headers(str(uuid.uuid4())))
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "INVOICE_NOT_FOUND"


def test_get_reflects_overdue_lazily(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = uuid.uuid4()
    row = fake_repo.seed(
        merchant_id=uid, status="OPEN", due_at=datetime.now(UTC) - timedelta(days=1)
    )
    r = client.get(f"/v1/invoices/{row.invoice_id}", headers=merchant_headers(str(uid)))
    assert r.json()["status"] == "OVERDUE"  # displayed
    assert row.status == "OPEN"  # persisted (until the sweeper runs)


def test_list_filters_by_status(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = uuid.uuid4()
    fake_repo.seed(merchant_id=uid, status="DRAFT")
    fake_repo.seed(merchant_id=uid, status="OPEN", pl_id="PLK_1")
    fake_repo.seed(merchant_id=uuid.uuid4(), status="OPEN", pl_id="PLK_other")  # other merchant
    r = client.get("/v1/invoices", params={"status": "OPEN"}, headers=merchant_headers(str(uid)))
    assert r.status_code == 200
    items = r.json()["items"]
    assert len(items) == 1
    assert items[0]["status"] == "OPEN"


def test_list_rejects_bad_status(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    r = client.get("/v1/invoices", params={"status": "BOGUS"}, headers=merchant_headers(uid))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_QUERY"
