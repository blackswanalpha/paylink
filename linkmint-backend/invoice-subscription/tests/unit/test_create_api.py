"""POST /v1/invoices — create a DRAFT invoice (validation + idempotency)."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

from fastapi.testclient import TestClient

from tests._support import FakeInvoiceRepository, create_body, merchant_headers


def test_create_returns_draft(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = str(uuid.uuid4())
    r = client.post("/v1/invoices", json=create_body(), headers=merchant_headers(uid))
    assert r.status_code == 201
    body = r.json()
    assert body["status"] == "DRAFT"
    assert body["pl_id"] is None
    assert uuid.UUID(body["invoice_id"])

    kinds = [k for (_i, k, _p) in fake_repo.events]
    assert "invoice.created" in kinds
    row = next(iter(fake_repo.invoices.values()))
    assert int(row.total) == 3480  # 2*1500 + 16% tax
    assert int(row.subtotal) == 3000
    assert int(row.tax) == 480
    assert row.merchant_id == uuid.UUID(uid)


def test_create_requires_auth(client: TestClient) -> None:
    assert client.post("/v1/invoices", json=create_body()).status_code == 401


def test_create_rejects_empty_lines(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    r = client.post("/v1/invoices", json=create_body(lines=[]), headers=merchant_headers(uid))
    assert r.status_code == 400


def test_create_rejects_bad_payee_addr(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    r = client.post(
        "/v1/invoices", json=create_body(payee_addr="not-an-addr"), headers=merchant_headers(uid)
    )
    assert r.status_code == 400


def test_create_rejects_past_due(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    past = (datetime.now(UTC) - timedelta(days=1)).isoformat()
    r = client.post("/v1/invoices", json=create_body(due_at=past), headers=merchant_headers(uid))
    assert r.status_code == 400


def test_create_defaults_currency(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = str(uuid.uuid4())
    body = create_body()
    body.pop("currency")
    r = client.post("/v1/invoices", json=body, headers=merchant_headers(uid))
    assert r.status_code == 201
    row = next(iter(fake_repo.invoices.values()))
    assert row.currency == "PLN"


def test_create_idempotent_replays(client: TestClient, fake_repo: FakeInvoiceRepository) -> None:
    uid = str(uuid.uuid4())
    headers = merchant_headers(uid)
    headers["Idempotency-Key"] = "key-1"
    body = create_body()
    r1 = client.post("/v1/invoices", json=body, headers=headers)
    r2 = client.post("/v1/invoices", json=body, headers=headers)
    assert r1.status_code == 201
    assert r2.status_code == 201
    assert r1.json()["invoice_id"] == r2.json()["invoice_id"]
    assert len(fake_repo.invoices) == 1  # replayed, not re-created
