"""Error envelope shape + health/readiness probes."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import auth_headers


def test_not_found_envelope(client: TestClient) -> None:
    r = client.get(
        f"/v1/pricing/merchants/{uuid.uuid4()}", headers=auth_headers(user_roles=["admin"])
    )
    assert r.status_code == 404
    err = r.json()["error"]
    assert err["code"] == "MERCHANT_PRICING_NOT_FOUND"
    assert set(err.keys()) == {"code", "message", "details", "trace_id"}


def test_validation_envelope(client: TestClient) -> None:
    # gross must be >= 1 → request validation failure → INVALID_PAYLOAD envelope.
    r = client.post(
        "/v1/pricing/quote",
        json={"merchant_id": str(uuid.uuid4()), "gross": 0, "currency": "KES", "rails": ["mpesa"]},
        headers=auth_headers(),
    )
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
