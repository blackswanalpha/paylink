"""`/internal/admin/users` read surface (consumed by admin-backoffice, work11)."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient


def _register(client: TestClient, email: str, password: str = "passw0rd123") -> str:
    resp = client.post("/v1/auth/register", json={"email": email, "password": password})
    assert resp.status_code == 201, resp.text
    return resp.json()["user_id"]


def test_get_user_by_id(client: TestClient) -> None:
    uid = _register(client, "admin-view@example.com")
    resp = client.get(f"/internal/admin/users/{uid}")
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["user_id"] == uid
    assert body["email"] == "admin-view@example.com"
    assert body["status"] == "ACTIVE"
    # Never leak secrets through the admin read surface.
    assert "password" not in body and "password_hash" not in body


def test_get_user_not_found(client: TestClient) -> None:
    resp = client.get(f"/internal/admin/users/{uuid.uuid4()}")
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "USER_NOT_FOUND"


def test_get_user_bad_uuid(client: TestClient) -> None:
    resp = client.get("/internal/admin/users/not-a-uuid")
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_search_users_by_email_substring(client: TestClient) -> None:
    _register(client, "searchme@example.com")
    _register(client, "other@example.com")
    resp = client.get("/internal/admin/users", params={"q": "searchme"})
    assert resp.status_code == 200, resp.text
    items = resp.json()["items"]
    assert [i["email"] for i in items] == ["searchme@example.com"]


def test_search_users_by_exact_id(client: TestClient) -> None:
    uid = _register(client, "byid@example.com")
    resp = client.get("/internal/admin/users", params={"q": uid})
    assert resp.status_code == 200
    assert resp.json()["items"][0]["user_id"] == uid


def test_search_users_no_match_is_empty(client: TestClient) -> None:
    resp = client.get("/internal/admin/users", params={"q": "nobody-here"})
    assert resp.status_code == 200
    assert resp.json()["items"] == []
