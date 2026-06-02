"""POST /v1/invoices/{id}/finalize — mint the PayLink, DRAFT → OPEN (one-way)."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import ADDR, FakeInvoiceRepository, FakePaylink, create_body, merchant_headers


def _create(client: TestClient, uid: str) -> str:
    r = client.post("/v1/invoices", json=create_body(), headers=merchant_headers(uid))
    return r.json()["invoice_id"]


def test_finalize_opens_and_mints_paylink(client: TestClient, fake_paylink: FakePaylink) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "OPEN"
    assert body["pl_id"] == "PLK_test"
    assert len(fake_paylink.calls) == 1
    call = fake_paylink.calls[0]
    assert call["amount"] == 3480
    assert call["receiver"] == ADDR
    assert call["usage"] == "single"
    # End-to-end idempotent mint: a stable per-invoice key is forwarded to paylink-service.
    assert call["idempotency_key"] == f"invoice-{iid}"


def test_finalize_is_one_way(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    assert (
        client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid)).status_code
        == 200
    )
    r2 = client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    assert r2.status_code == 409
    assert r2.json()["error"]["code"] == "INVALID_STATE"


def test_finalize_paylink_failure_is_502(client: TestClient, fake_paylink: FakePaylink) -> None:
    fake_paylink.fail = True
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    assert r.status_code == 502
    assert r.json()["error"]["code"] == "PAYLINK_UNAVAILABLE"


def test_finalize_no_plid_is_502(client: TestClient, fake_paylink: FakePaylink) -> None:
    fake_paylink.no_plid = True
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(uid))
    assert r.status_code == 502


def test_finalize_other_merchant_404(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    iid = _create(client, uid)
    r = client.post(f"/v1/invoices/{iid}/finalize", headers=merchant_headers(str(uuid.uuid4())))
    assert r.status_code == 404


def test_finalize_then_void_blocked_when_paid(
    client: TestClient, fake_repo: FakeInvoiceRepository
) -> None:
    uid = uuid.uuid4()
    row = fake_repo.seed(merchant_id=uid, status="PAID", pl_id="PLK_paid")
    r = client.post(f"/v1/invoices/{row.invoice_id}/void", headers=merchant_headers(str(uid)))
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "ALREADY_PAID"
