"""End-to-end flow against REAL Postgres + Redis — the acceptance round-trip + RBAC + idempotency."""

from __future__ import annotations

import uuid

import pyotp
import pytest
from fastapi.testclient import TestClient

pytestmark = pytest.mark.integration

PW = "passw0rd123"


def test_register_login_refresh_me_roundtrip(live_client: TestClient) -> None:
    c = live_client
    reg = c.post("/v1/auth/register", json={"email": "int@x.com", "password": PW})
    assert reg.status_code == 201, reg.text

    tokens = c.post("/v1/auth/login", json={"email": "int@x.com", "password": PW}).json()
    headers = {"Authorization": f"Bearer {tokens['access_token']}"}

    me = c.get("/v1/users/me", headers=headers)
    assert me.status_code == 200
    assert me.json()["email"] == "int@x.com"

    rotated = c.post("/v1/auth/refresh", json={"refresh_token": tokens["refresh_token"]})
    assert rotated.status_code == 200
    assert rotated.json()["refresh_token"] != tokens["refresh_token"]
    # old refresh token is now revoked → reuse detected
    assert (
        c.post("/v1/auth/refresh", json={"refresh_token": tokens["refresh_token"]}).status_code
        == 401
    )


def test_org_members_api_keys_sessions(live_client: TestClient) -> None:
    c = live_client
    c.post("/v1/auth/register", json={"email": "owner-int@x.com", "password": PW})
    tokens = c.post("/v1/auth/login", json={"email": "owner-int@x.com", "password": PW}).json()
    headers = {"Authorization": f"Bearer {tokens['access_token']}"}

    org = c.post("/v1/organizations", headers=headers, json={"name": "Acme", "type": "merchant"})
    assert org.status_code == 201, org.text
    org_id = org.json()["org_id"]

    issued = c.post(
        "/v1/users/me/api-keys",
        headers=headers,
        json={"org_id": org_id, "name": "ci", "scopes": ["paylinks:read", "paylinks:write"]},
    )
    assert issued.status_code == 201, issued.text
    key_id = issued.json()["api_key_id"]
    assert issued.json()["full_key"].startswith("lm_live_")
    listed = c.get("/v1/users/me/api-keys", headers=headers).json()["items"]
    assert any(i["api_key_id"] == key_id and "full_key" not in i for i in listed)
    assert c.delete(f"/v1/users/me/api-keys/{key_id}", headers=headers).status_code == 200

    member = c.post("/v1/auth/register", json={"email": "member-int@x.com", "password": PW})
    member_id = member.json()["user_id"]
    invited = c.post(
        f"/v1/organizations/{org_id}/members",
        headers=headers,
        json={"user_id": member_id, "role": "developer"},
    )
    assert invited.status_code == 201, invited.text
    assert len(c.get(f"/v1/organizations/{org_id}/members", headers=headers).json()["items"]) == 2

    sessions = c.get("/v1/sessions", headers=headers).json()["items"]
    assert len(sessions) >= 1 and any(s["current"] for s in sessions)


def test_mfa_enroll_verify_against_real_db(live_client: TestClient) -> None:
    c = live_client
    c.post("/v1/auth/register", json={"email": "mfa-int@x.com", "password": PW})
    tokens = c.post("/v1/auth/login", json={"email": "mfa-int@x.com", "password": PW}).json()
    headers = {"Authorization": f"Bearer {tokens['access_token']}"}
    secret = c.post("/v1/auth/mfa/enroll", headers=headers).json()["secret"]
    verified = c.post(
        "/v1/auth/mfa/verify", headers=headers, json={"code": pyotp.TOTP(secret).now()}
    )
    assert verified.status_code == 200
    # login now requires the code
    blocked = c.post("/v1/auth/login", json={"email": "mfa-int@x.com", "password": PW})
    assert blocked.status_code == 401
    assert blocked.json()["error"]["code"] == "MFA_REQUIRED"


def test_idempotent_register_against_real_redis(live_client: TestClient) -> None:
    c = live_client
    key = {"Idempotency-Key": "int-reg-1"}
    a = c.post("/v1/auth/register", json={"email": "idem-int@x.com", "password": PW}, headers=key)
    b = c.post("/v1/auth/register", json={"email": "idem-int@x.com", "password": PW}, headers=key)
    assert a.status_code == 201 and b.status_code == 201
    assert a.json() == b.json()


def test_kyc_consumer_updates_tier(live_client: TestClient, pg_url: str) -> None:
    """The compliance.kyc.* consumer seam writes through to the real DB."""
    import asyncio

    from app.db.repositories import IdentityRepository
    from app.db.session import make_engine, make_sessionmaker
    from app.domain.users_service import UsersService
    from app.events.consumer import KYC_PASSED, KycConsumer
    from app.events.stub import NoopPublisher

    c = live_client
    user_id = c.post("/v1/auth/register", json={"email": "kyc-int@x.com", "password": PW}).json()[
        "user_id"
    ]

    async def _drive() -> int:
        engine = make_engine(pg_url)
        sm = make_sessionmaker(engine)
        try:
            async with sm() as session:
                users = UsersService(IdentityRepository(session), session.commit, NoopPublisher())
                await KycConsumer(users).handle(KYC_PASSED, {"user_id": user_id, "tier": 2})
                fetched = await IdentityRepository(session).get_user(uuid.UUID(user_id))
                return fetched.kyc_tier if fetched else -1
        finally:
            await engine.dispose()

    assert asyncio.run(_drive()) == 2
