"""Health/readiness endpoints, the error envelope, and config parsing."""

from __future__ import annotations

from fastapi.testclient import TestClient

from tests._support import auth_headers, make_settings


def test_healthz(client: TestClient) -> None:
    r = client.get("/internal/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_readyz_reports_checks(client: TestClient) -> None:
    # No real DB/Redis behind the TestClient → readiness reports not_ready (503), never raises.
    r = client.get("/internal/readyz")
    assert r.status_code in (200, 503)
    assert "checks" in r.json()


def test_unknown_route_envelope(client: TestClient) -> None:
    r = client.get("/v1/nope", headers=auth_headers())
    assert r.status_code == 404
    assert "error" in r.json()


def test_validation_error_envelope(client: TestClient) -> None:
    # amount_minor must be > 0
    r = client.post(
        "/v1/refunds", headers=auth_headers(), json={"payment_id": "p", "amount_minor": -1}
    )
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_webhook_secrets_map_parsing() -> None:
    s = make_settings(webhook_secrets="stub:a;stripe:b; :skip;malformed")
    assert s.webhook_secrets_map == {"stub": "a", "stripe": "b"}


def test_admin_role_set_parsing() -> None:
    s = make_settings(admin_user_roles="admin, ops ,")
    assert s.admin_user_role_set == frozenset({"admin", "ops"})
