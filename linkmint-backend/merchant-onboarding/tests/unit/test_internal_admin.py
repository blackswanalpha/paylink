"""`/internal/admin/merchants` read surface (consumed by admin-backoffice, work11)."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import auth_headers_for


def _onboard(client: TestClient, org_id: str, name: str = "Acme Ltd") -> str:
    resp = client.post(
        "/v1/merchants/onboard",
        json={"org_id": org_id, "business_name": name, "country": "KE", "type": "company"},
        headers=auth_headers_for(org_id),
    )
    assert resp.status_code == 201, resp.text
    return resp.json()["merchant_id"]


def test_admin_get_merchant_bypasses_org_rbac(client: TestClient) -> None:
    org_id = str(uuid.uuid4())
    mid = _onboard(client, org_id, "Acme Ltd")
    # No Authorization header — the internal surface is gateway-authorized, not org-RBAC-gated.
    resp = client.get(f"/internal/admin/merchants/{mid}")
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["merchant_id"] == mid
    assert body["org_id"] == org_id
    assert body["business_name"] == "Acme Ltd"
    assert body["bank_accounts"] == []
    assert "account_ref" not in str(body)  # never leak the at-rest ciphertext


def test_admin_get_merchant_not_found(client: TestClient) -> None:
    resp = client.get(f"/internal/admin/merchants/{uuid.uuid4()}")
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "MERCHANT_NOT_FOUND"


def test_admin_get_merchant_bad_uuid(client: TestClient) -> None:
    resp = client.get("/internal/admin/merchants/not-a-uuid")
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_admin_search_by_business_name(client: TestClient) -> None:
    _onboard(client, str(uuid.uuid4()), "Globex Corporation")
    _onboard(client, str(uuid.uuid4()), "Initech")
    resp = client.get("/internal/admin/merchants", params={"q": "globex"})
    assert resp.status_code == 200, resp.text
    assert [i["business_name"] for i in resp.json()["items"]] == ["Globex Corporation"]


def test_admin_search_by_org_id(client: TestClient) -> None:
    org_id = str(uuid.uuid4())
    mid = _onboard(client, org_id, "Soylent")
    resp = client.get("/internal/admin/merchants", params={"q": org_id})
    assert resp.status_code == 200
    assert resp.json()["items"][0]["merchant_id"] == mid


def test_admin_search_no_match_is_empty(client: TestClient) -> None:
    resp = client.get("/internal/admin/merchants", params={"q": "nobody"})
    assert resp.status_code == 200
    assert resp.json()["items"] == []
