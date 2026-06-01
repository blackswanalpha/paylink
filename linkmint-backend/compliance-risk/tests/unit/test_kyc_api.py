from __future__ import annotations

import json
import uuid

from fastapi.testclient import TestClient

from app.security.hmac import compute_signature
from tests._support import CALLBACK_SECRET, FakeRepository, user_headers

SESSIONS = "/v1/kyc/sessions"


def _callback_url(provider: str = "stub") -> str:
    return f"/v1/kyc/callbacks/{provider}"


# ── create session ──
def test_create_session_201_self(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    resp = client.post(
        SESSIONS,
        json={"user_id": uid, "tier_requested": 2},
        headers=user_headers(uid),
    )
    assert resp.status_code == 201
    body = resp.json()
    assert body["session_id"]
    assert body["provider_url"].startswith("https://kyc.stub.local/s/")


def test_create_session_requires_auth(client: TestClient) -> None:
    resp = client.post(SESSIONS, json={"user_id": str(uuid.uuid4()), "tier_requested": 1})
    assert resp.status_code == 401


def test_create_session_other_user_forbidden_unless_admin(client: TestClient) -> None:
    caller = str(uuid.uuid4())
    other = str(uuid.uuid4())
    resp = client.post(
        SESSIONS, json={"user_id": other, "tier_requested": 1}, headers=user_headers(caller)
    )
    assert resp.status_code == 403
    assert resp.json()["error"]["code"] == "FORBIDDEN"


def test_create_session_admin_for_other_user_ok(client: TestClient) -> None:
    admin = str(uuid.uuid4())
    other = str(uuid.uuid4())
    resp = client.post(
        SESSIONS,
        json={"user_id": other, "tier_requested": 1},
        headers=user_headers(admin, admin=True),
    )
    assert resp.status_code == 201


def test_create_session_invalid_tier_422(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    resp = client.post(
        SESSIONS, json={"user_id": uid, "tier_requested": 5}, headers=user_headers(uid)
    )
    assert resp.status_code == 400  # pydantic ge/le → INVALID_PAYLOAD envelope


def test_create_session_already_verified_409(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())
    fake_repo.seed_kyc(uuid.UUID(uid), tier=2)
    resp = client.post(
        SESSIONS, json={"user_id": uid, "tier_requested": 2}, headers=user_headers(uid)
    )
    assert resp.status_code == 409
    assert resp.json()["error"]["code"] == "ALREADY_VERIFIED"


def test_create_session_idempotent_replay(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    headers = {**user_headers(uid), "Idempotency-Key": "k-1"}
    body = {"user_id": uid, "tier_requested": 2}
    r1 = client.post(SESSIONS, json=body, headers=headers)
    r2 = client.post(SESSIONS, json=body, headers=headers)
    assert r1.status_code == 201 and r2.status_code == 201
    assert r1.json() == r2.json()  # same cached response


# ── callbacks (HMAC, not JWT) ──
def test_callback_ok_with_valid_signature(client: TestClient, fake_repo: FakeRepository) -> None:
    uid = str(uuid.uuid4())
    raw = json.dumps({"user_id": uid, "status": "passed", "tier": 2}).encode()
    sig = compute_signature(CALLBACK_SECRET, raw)
    resp = client.post(_callback_url(), content=raw, headers={"X-Signature": sig})
    assert resp.status_code == 200
    assert resp.json() == {"ok": True}
    assert fake_repo.kyc[uuid.UUID(uid)].tier == 2


def test_callback_accepts_sha256_prefixed_signature(
    client: TestClient, fake_repo: FakeRepository
) -> None:
    uid = str(uuid.uuid4())
    raw = json.dumps({"user_id": uid, "status": "passed"}).encode()
    sig = compute_signature(CALLBACK_SECRET, raw)
    resp = client.post(_callback_url(), content=raw, headers={"X-Signature": f"sha256={sig}"})
    assert resp.status_code == 200


def test_callback_bad_signature_401(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    raw = json.dumps({"user_id": uid, "status": "passed"}).encode()
    resp = client.post(_callback_url(), content=raw, headers={"X-Signature": "deadbeef"})
    assert resp.status_code == 401
    assert resp.json()["error"]["code"] == "INVALID_SIGNATURE"


def test_callback_missing_signature_401(client: TestClient) -> None:
    uid = str(uuid.uuid4())
    raw = json.dumps({"user_id": uid, "status": "passed"}).encode()
    resp = client.post(_callback_url(), content=raw)
    assert resp.status_code == 401


def test_callback_unknown_provider_404(client: TestClient) -> None:
    raw = json.dumps({"user_id": str(uuid.uuid4()), "status": "passed"}).encode()
    sig = compute_signature(CALLBACK_SECRET, raw)
    resp = client.post(_callback_url("nope"), content=raw, headers={"X-Signature": sig})
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "UNKNOWN_PROVIDER"


def test_callback_no_jwt_required(client: TestClient, fake_repo: FakeRepository) -> None:
    # Callbacks carry NO bearer token — the HMAC is the trust anchor.
    uid = str(uuid.uuid4())
    raw = json.dumps({"user_id": uid, "status": "failed"}).encode()
    sig = compute_signature(CALLBACK_SECRET, raw)
    resp = client.post(_callback_url(), content=raw, headers={"X-Signature": sig})
    assert resp.status_code == 200
    assert fake_repo.kyc[uuid.UUID(uid)].tier == 0
