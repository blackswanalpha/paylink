"""GET /v1/pricing/tiers — platform-admin gate + listing."""

from __future__ import annotations

from fastapi.testclient import TestClient

from tests._support import auth_headers


def test_tiers_forbidden_for_non_admin(client: TestClient) -> None:
    r = client.get("/v1/pricing/tiers", headers=auth_headers(user_roles=["merchant"]))
    assert r.status_code == 403
    assert r.json()["error"]["code"] == "FORBIDDEN"


def test_tiers_listed_for_admin(client: TestClient) -> None:
    r = client.get("/v1/pricing/tiers", headers=auth_headers(user_roles=["admin"]))
    assert r.status_code == 200
    tiers = r.json()["tiers"]
    assert {t["tier"] for t in tiers} == {"standard", "startup", "growth", "scale", "enterprise"}
    standard = next(t for t in tiers if t["tier"] == "standard")
    assert standard["platform_pct_bps"] == 250


def test_tiers_requires_auth(client: TestClient) -> None:
    assert client.get("/v1/pricing/tiers").status_code == 401
