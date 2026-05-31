"""End-to-end API tests over in-memory fakes — real RS256 JWT, argon2, RBAC, and idempotency."""

from __future__ import annotations

import uuid

import pyotp
from fastapi.testclient import TestClient

from tests.conftest import auth_headers, login, register

PW = "passw0rd123"


# ── ops ──
def test_healthz(client: TestClient) -> None:
    assert client.get("/internal/healthz").json() == {"status": "ok"}


def test_readyz_reports_without_raising(client: TestClient) -> None:
    r = client.get("/internal/readyz")
    assert r.status_code in (200, 503)
    assert "checks" in r.json()


def test_metrics(client: TestClient) -> None:
    assert client.get("/metrics").status_code == 200


# ── register / login / me ──
def test_register_then_duplicate_email(client: TestClient) -> None:
    assert register(client, "a@b.com")
    r = client.post("/v1/auth/register", json={"email": "a@b.com", "password": PW})
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "EMAIL_TAKEN"


def test_register_requires_identifier(client: TestClient) -> None:
    r = client.post("/v1/auth/register", json={"password": PW})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_login_bad_password(client: TestClient) -> None:
    register(client, "c@d.com")
    r = client.post("/v1/auth/login", json={"email": "c@d.com", "password": "wrong"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_CREDENTIALS"


def test_login_me_roundtrip(client: TestClient) -> None:
    headers = auth_headers(client, "me@x.com")
    body = client.get("/v1/users/me", headers=headers).json()
    assert body["email"] == "me@x.com"
    assert body["status"] == "ACTIVE"
    assert "payer" in body["user_roles"]


def test_me_requires_auth(client: TestClient) -> None:
    r = client.get("/v1/users/me")
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "UNAUTHORIZED"


def test_me_rejects_garbage_token(client: TestClient) -> None:
    r = client.get("/v1/users/me", headers={"Authorization": "Bearer not.a.jwt"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_TOKEN"


def test_update_profile(client: TestClient) -> None:
    headers = auth_headers(client, "upd@x.com")
    r = client.patch("/v1/users/me", headers=headers, json={"phone": "+254700000000"})
    assert r.status_code == 200
    assert r.json()["phone"] == "+254700000000"


# ── refresh rotation + reuse detection ──
def test_refresh_rotation_and_reuse_detection(client: TestClient) -> None:
    register(client, "r@x.com")
    rt = login(client, "r@x.com")["refresh_token"]
    rotated = client.post("/v1/auth/refresh", json={"refresh_token": rt})
    assert rotated.status_code == 200
    new_rt = rotated.json()["refresh_token"]
    assert new_rt != rt
    # replaying the rotated-away token → reuse detected → 401
    reused = client.post("/v1/auth/refresh", json={"refresh_token": rt})
    assert reused.status_code == 401
    assert reused.json()["error"]["code"] == "INVALID_TOKEN"
    # the whole family is revoked, so the new token is dead too
    assert client.post("/v1/auth/refresh", json={"refresh_token": new_rt}).status_code == 401


def test_logout_revokes_refresh(client: TestClient) -> None:
    register(client, "lo@x.com")
    tokens = login(client, "lo@x.com")
    headers = {"Authorization": f"Bearer {tokens['access_token']}"}
    out = client.post(
        "/v1/auth/logout", headers=headers, json={"refresh_token": tokens["refresh_token"]}
    )
    assert out.status_code == 200
    assert (
        client.post("/v1/auth/refresh", json={"refresh_token": tokens["refresh_token"]}).status_code
        == 401
    )


# ── JWKS / OIDC ──
def test_jwks(client: TestClient) -> None:
    r = client.get("/v1/auth/.well-known/jwks.json")
    assert r.status_code == 200
    assert r.json()["keys"][0]["alg"] == "RS256"


def test_oidc_metadata(client: TestClient) -> None:
    r = client.get("/v1/auth/.well-known/openid-configuration")
    assert r.status_code == 200
    assert r.json()["issuer"] == "linkmint-identity"


# ── MFA ──
def _enroll_activate(client: TestClient, headers: dict[str, str]) -> str:
    secret = client.post("/v1/auth/mfa/enroll", headers=headers).json()["secret"]
    code = pyotp.TOTP(secret).now()
    v = client.post("/v1/auth/mfa/verify", headers=headers, json={"code": code})
    assert v.status_code == 200 and v.json()["enabled"] is True
    return secret


def test_mfa_enroll_verify_then_login_requires_code(client: TestClient) -> None:
    register(client, "mfa@x.com")
    tokens = login(client, "mfa@x.com")
    headers = {"Authorization": f"Bearer {tokens['access_token']}"}
    secret = _enroll_activate(client, headers)
    blocked = client.post("/v1/auth/login", json={"email": "mfa@x.com", "password": PW})
    assert blocked.status_code == 401
    assert blocked.json()["error"]["code"] == "MFA_REQUIRED"
    ok = client.post(
        "/v1/auth/login",
        json={"email": "mfa@x.com", "password": PW, "mfa_code": pyotp.TOTP(secret).now()},
    )
    assert ok.status_code == 200


def test_mfa_verify_wrong_code(client: TestClient) -> None:
    register(client, "mfa2@x.com")
    headers = {"Authorization": f"Bearer {login(client, 'mfa2@x.com')['access_token']}"}
    client.post("/v1/auth/mfa/enroll", headers=headers)
    r = client.post("/v1/auth/mfa/verify", headers=headers, json={"code": "000000"})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "MFA_INVALID"


# ── orgs / api keys ──
def _create_org(client: TestClient, headers: dict[str, str], name: str = "Acme") -> dict:
    r = client.post("/v1/organizations", headers=headers, json={"name": name, "type": "merchant"})
    assert r.status_code == 201, r.text
    return r.json()


def test_org_create_then_api_key_lifecycle(client: TestClient) -> None:
    headers = auth_headers(client, "owner@x.com")
    org = _create_org(client, headers)
    assert org["role"] == "owner"
    issued = client.post(
        "/v1/users/me/api-keys",
        headers=headers,
        json={"org_id": org["org_id"], "name": "ci", "scopes": ["paylinks:read", "paylinks:write"]},
    )
    assert issued.status_code == 201, issued.text
    body = issued.json()
    assert body["full_key"].startswith("lm_live_")
    assert body["status"] == "ACTIVE"
    listed = client.get("/v1/users/me/api-keys", headers=headers).json()["items"]
    assert any(i["api_key_id"] == body["api_key_id"] for i in listed)
    assert all("full_key" not in i and "hash" not in i for i in listed)
    revoked = client.delete(f"/v1/users/me/api-keys/{body['api_key_id']}", headers=headers)
    assert revoked.status_code == 200 and revoked.json()["status"] == "REVOKED"


def test_api_key_unknown_scope_rejected(client: TestClient) -> None:
    headers = auth_headers(client, "owner2@x.com")
    org = _create_org(client, headers, "Org2")
    r = client.post(
        "/v1/users/me/api-keys",
        headers=headers,
        json={"org_id": org["org_id"], "name": "x", "scopes": ["bogus"]},
    )
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


# ── members + RBAC ──
def test_member_invite_list_and_viewer_forbidden(client: TestClient) -> None:
    owner_h = auth_headers(client, "o@x.com")
    org_id = _create_org(client, owner_h)["org_id"]
    member_id = register(client, "m@x.com")
    invited = client.post(
        f"/v1/organizations/{org_id}/members",
        headers=owner_h,
        json={"user_id": member_id, "role": "viewer"},
    )
    assert invited.status_code == 201, invited.text
    members = client.get(f"/v1/organizations/{org_id}/members", headers=owner_h).json()["items"]
    assert len(members) == 2
    member_h = {"Authorization": f"Bearer {login(client, 'm@x.com')['access_token']}"}
    third = register(client, "n@x.com")
    forbidden = client.post(
        f"/v1/organizations/{org_id}/members",
        headers=member_h,
        json={"user_id": third, "role": "viewer"},
    )
    assert forbidden.status_code == 403
    assert forbidden.json()["error"]["code"] == "FORBIDDEN"


def test_remove_member_and_last_owner_guard(client: TestClient) -> None:
    owner_h = auth_headers(client, "o2@x.com")
    org_id = _create_org(client, owner_h, "Org3")["org_id"]
    me = client.get("/v1/users/me", headers=owner_h).json()
    last_owner = client.delete(
        f"/v1/organizations/{org_id}/members/{me['user_id']}", headers=owner_h
    )
    assert last_owner.status_code == 409
    assert last_owner.json()["error"]["code"] == "CANNOT_REMOVE_LAST_OWNER"
    member_id = register(client, "rm@x.com")
    client.post(
        f"/v1/organizations/{org_id}/members",
        headers=owner_h,
        json={"user_id": member_id, "role": "viewer"},
    )
    removed = client.delete(f"/v1/organizations/{org_id}/members/{member_id}", headers=owner_h)
    assert removed.status_code == 200


def test_invite_unknown_user(client: TestClient) -> None:
    owner_h = auth_headers(client, "o3@x.com")
    org_id = _create_org(client, owner_h, "Org4")["org_id"]
    r = client.post(
        f"/v1/organizations/{org_id}/members",
        headers=owner_h,
        json={"user_id": str(uuid.uuid4()), "role": "viewer"},
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "MEMBER_NOT_FOUND"


def test_org_endpoints_require_membership(client: TestClient) -> None:
    outsider_h = auth_headers(client, "out@x.com")
    r = client.get(f"/v1/organizations/{uuid.uuid4()}/members", headers=outsider_h)
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "ORG_NOT_FOUND"


# ── sessions ──
def test_sessions_list_and_revoke(client: TestClient) -> None:
    register(client, "s@x.com")
    headers = {"Authorization": f"Bearer {login(client, 's@x.com')['access_token']}"}
    items = client.get("/v1/sessions", headers=headers).json()["items"]
    current = [i for i in items if i["current"]]
    assert len(current) == 1
    revoked = client.delete(f"/v1/sessions/{current[0]['session_id']}", headers=headers)
    assert revoked.status_code == 200


# ── OAuth (fake) ──
def test_oauth_fake_flow(client: TestClient) -> None:
    state = client.post("/v1/auth/oauth/google/start", json={}).json()["state"]
    cb = client.post("/v1/auth/oauth/google/callback", json={"code": "abc", "state": state})
    assert cb.status_code == 200
    assert cb.json()["access_token"]


def test_oauth_unknown_provider(client: TestClient) -> None:
    r = client.post("/v1/auth/oauth/twitter/start", json={})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "OAUTH_PROVIDER_UNKNOWN"


def test_oauth_does_not_merge_into_existing_email_account(client: TestClient) -> None:
    """SECURITY: an OAuth login whose email collides with a password account must NOT take it over."""
    import hashlib

    subject = hashlib.sha256(b"google:abc").hexdigest()[:32]
    collide_email = f"{subject}@fake-google.local"  # the fake provider's deterministic email
    pwd_uid = register(client, collide_email)

    state = client.post("/v1/auth/oauth/google/start", json={}).json()["state"]
    cb = client.post("/v1/auth/oauth/google/callback", json={"code": "abc", "state": state})
    assert cb.status_code == 200
    me = client.get(
        "/v1/users/me", headers={"Authorization": f"Bearer {cb.json()['access_token']}"}
    ).json()
    assert me["user_id"] != pwd_uid  # a fresh account, not the victim's
    assert me["email"] is None  # email was taken → left null on the OAuth account
    # the password account is untouched and still usable
    assert (
        client.post("/v1/auth/login", json={"email": collide_email, "password": PW}).status_code
        == 200
    )


# ── idempotency ──
def test_idempotent_register_replay(client: TestClient) -> None:
    headers = {"Idempotency-Key": "reg-1"}
    first = client.post(
        "/v1/auth/register", json={"email": "idem@x.com", "password": PW}, headers=headers
    )
    second = client.post(
        "/v1/auth/register", json={"email": "idem@x.com", "password": PW}, headers=headers
    )
    assert first.status_code == 201
    assert second.status_code == 201
    assert first.json() == second.json()  # replay, not EMAIL_TAKEN


def test_idempotent_conflict_on_body_change(client: TestClient) -> None:
    headers = {"Idempotency-Key": "reg-2"}
    client.post("/v1/auth/register", json={"email": "i2@x.com", "password": PW}, headers=headers)
    r = client.post(
        "/v1/auth/register", json={"email": "other@x.com", "password": PW}, headers=headers
    )
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "IDEMPOTENT_CONFLICT"


def test_api_key_issue_redacts_full_key_on_idempotent_replay(client: TestClient) -> None:
    """SECURITY: the one-time secret must never be cached — replay returns full_key=null, same id."""
    headers = auth_headers(client, "akrep@x.com")
    org = _create_org(client, headers, "AKOrg")
    body = {"org_id": org["org_id"], "name": "ci", "scopes": ["paylinks:read"]}
    k = {"Idempotency-Key": "ak-1", **headers}
    first = client.post("/v1/users/me/api-keys", headers=k, json=body)
    second = client.post("/v1/users/me/api-keys", headers=k, json=body)
    assert first.status_code == 201 and first.json()["full_key"].startswith("lm_live_")
    assert second.status_code == 201
    assert second.json()["full_key"] is None  # secret not cached/replayed
    assert second.json()["api_key_id"] == first.json()["api_key_id"]  # same key, no duplicate
