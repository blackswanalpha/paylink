from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import FakeRepository, user_headers

STATUS = "/v1/compliance/status"


def test_status_self_ok(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=2)
    resp = client.get(STATUS, params={"user_id": uid}, headers=user_headers(uid))
    assert resp.status_code == 200
    body = resp.json()
    assert body["user_id"] == uid
    assert body["kyc_tier"] == 2
    assert body["risk_score"] is None
    assert body["flags"] == []


def test_status_requires_auth(client: TestClient) -> None:
    resp = client.get(STATUS, params={"user_id": str(uuid.uuid4())})
    assert resp.status_code == 401


def test_status_other_user_forbidden(client: TestClient, fake_repo: FakeRepository) -> None:
    caller = str(uuid.uuid4())
    other = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(other), tier=1)
    resp = client.get(STATUS, params={"user_id": other}, headers=user_headers(caller))
    assert resp.status_code == 403
    assert resp.json()["error"]["code"] == "FORBIDDEN"


def test_status_admin_for_other_user_ok(client: TestClient, fake_repo: FakeRepository) -> None:
    admin = str(uuid.uuid4())
    other = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(other), tier=1)
    resp = client.get(STATUS, params={"user_id": other}, headers=user_headers(admin, admin=True))
    assert resp.status_code == 200
    assert resp.json()["kyc_tier"] == 1


def test_status_404_when_unknown(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    resp = client.get(STATUS, params={"user_id": uid}, headers=user_headers(uid))
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "COMPLIANCE_NOT_FOUND"


def test_status_bad_user_id_400(client: TestClient) -> None:
    # A non-UUID that passes self-check (caller==value) then fails parse_uuid.
    resp = client.get(STATUS, params={"user_id": "not-a-uuid"}, headers=user_headers("not-a-uuid"))
    assert resp.status_code == 400
    assert resp.json()["error"]["code"] == "INVALID_PAYLOAD"
