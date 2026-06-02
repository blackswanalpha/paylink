"""Error envelope shape + health/readiness probes."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import merchant_headers


def test_not_found_envelope(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    r = client.get(f"/v1/invoices/{uuid.uuid4()}", headers=merchant_headers(uid))
    assert r.status_code == 404
    err = r.json()["error"]
    assert err["code"] == "INVOICE_NOT_FOUND"
    assert set(err.keys()) == {"code", "message", "details", "trace_id"}


def test_invalid_uuid_path(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    r = client.get("/v1/invoices/not-a-uuid", headers=merchant_headers(uid))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_healthz(client: TestClient) -> None:
    r = client.get("/internal/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_readyz_reports_checks(client: TestClient) -> None:
    # No real DB/Redis in the unit env → readyz executes its checks and reports (200 or 503).
    r = client.get("/internal/readyz")
    assert r.status_code in (200, 503)
    assert "checks" in r.json()
