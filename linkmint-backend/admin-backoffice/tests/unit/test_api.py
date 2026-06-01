from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import (
    SAMPLE_MERCHANT_ID,
    SAMPLE_PAYLINK_ID,
    SAMPLE_PAYMENT_ID,
    SAMPLE_USER_ID,
    FakeAdminRepository,
    mint_token,
    staff_headers,
)

SEARCH = "/v1/admin/search"


def test_healthz(client: TestClient) -> None:
    assert client.get("/internal/healthz").json() == {"status": "ok"}


# ── gate matrix ──
def test_search_missing_token_401(client: TestClient) -> None:
    assert client.get(SEARCH, params={"q": "alice"}).status_code == 401


def test_search_malformed_auth_header_401(client: TestClient) -> None:
    resp = client.get(SEARCH, params={"q": "alice"}, headers={"Authorization": "Token xyz"})
    assert resp.status_code == 401


def test_search_expired_token_401(client: TestClient) -> None:
    headers = {"Authorization": f"Bearer {mint_token(expired=True)}"}
    resp = client.get(SEARCH, params={"q": "alice"}, headers=headers)
    assert resp.status_code == 401 and resp.json()["error"]["code"] == "TOKEN_EXPIRED"


def test_search_non_admin_403(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo, admin=False)
    resp = client.get(SEARCH, params={"q": "alice"}, headers=headers)
    assert resp.status_code == 403 and resp.json()["error"]["code"] == "FORBIDDEN"


def test_search_admin_without_mfa_403(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo, mfa=False)
    resp = client.get(SEARCH, params={"q": "alice"}, headers=headers)
    assert resp.status_code == 403 and resp.json()["error"]["code"] == "MFA_REQUIRED"


def test_search_admin_mfa_without_scope_403(
    client: TestClient, admin_repo: FakeAdminRepository
) -> None:
    headers = staff_headers(admin_repo, scopes=())  # admin + MFA but no grant
    resp = client.get(SEARCH, params={"q": "alice"}, headers=headers)
    assert resp.status_code == 403 and resp.json()["error"]["code"] == "SCOPE_DENIED"


def test_search_ok(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo)
    resp = client.get(SEARCH, params={"q": "alice"}, headers=headers)
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["degraded"] == []
    assert any(h["label"] == "alice@example.com" for h in body["groups"]["user"])


def test_search_superuser_grant_ok(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo, scopes=("superuser",))
    assert client.get(SEARCH, params={"q": "acme"}, headers=headers).status_code == 200


def test_search_short_query_400(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo)
    resp = client.get(SEARCH, params={"q": "a"}, headers=headers)
    assert resp.status_code == 400 and resp.json()["error"]["code"] == "INVALID_QUERY"


# ── entity views ──
def test_entity_views_ok(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo)
    for path, eid in (
        ("users", SAMPLE_USER_ID),
        ("merchants", SAMPLE_MERCHANT_ID),
        ("paylinks", SAMPLE_PAYLINK_ID),
        ("payments", SAMPLE_PAYMENT_ID),
    ):
        resp = client.get(f"/v1/admin/{path}/{eid}", headers=headers)
        assert resp.status_code == 200, resp.text
        assert resp.json()["id"] == eid


def test_entity_view_not_found_404(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo)
    resp = client.get(f"/v1/admin/users/{uuid.uuid4()}", headers=headers)
    assert resp.status_code == 404 and resp.json()["error"]["code"] == "ENTITY_NOT_FOUND"


def test_entity_view_requires_scope(client: TestClient, admin_repo: FakeAdminRepository) -> None:
    headers = staff_headers(admin_repo, scopes=())
    resp = client.get(f"/v1/admin/users/{SAMPLE_USER_ID}", headers=headers)
    assert resp.status_code == 403 and resp.json()["error"]["code"] == "SCOPE_DENIED"
