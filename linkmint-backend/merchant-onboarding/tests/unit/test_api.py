"""End-to-end API tests over in-memory fakes — real RS256 verify, AES-GCM, RBAC, and idempotency."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests.conftest import auth_headers


def _onboard(client: TestClient, headers: dict[str, str], org_id: str) -> str:
    r = client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={
            "org_id": org_id,
            "business_name": "Acme Ltd",
            "registration_no": "C.12345",
            "country": "KE",
            "type": "company",
        },
    )
    assert r.status_code == 201, r.text
    body = r.json()
    assert body["status"] == "PENDING_VERIFICATION"
    return body["merchant_id"]


# ── ops ──
def test_healthz(client: TestClient) -> None:
    assert client.get("/internal/healthz").json() == {"status": "ok"}


def test_readyz_reports_without_raising(client: TestClient) -> None:
    r = client.get("/internal/readyz")
    assert r.status_code in (200, 503)
    assert "checks" in r.json()


def test_metrics(client: TestClient) -> None:
    assert client.get("/metrics").status_code == 200


# ── auth surface ──
def test_onboard_requires_auth(client: TestClient) -> None:
    r = client.post("/v1/merchants/onboard", json={})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "UNAUTHORIZED"


def test_onboard_rejects_garbage_token(client: TestClient) -> None:
    r = client.post(
        "/v1/merchants/onboard",
        headers={"Authorization": "Bearer not.a.jwt"},
        json={
            "org_id": str(uuid.uuid4()),
            "business_name": "X",
            "country": "KE",
            "type": "company",
        },
    )
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_TOKEN"


# ── onboard ──
def test_onboard_then_duplicate(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    _onboard(client, headers, org)
    dup = client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={"org_id": org, "business_name": "Acme", "country": "KE", "type": "company"},
    )
    assert dup.status_code == 409
    assert dup.json()["error"]["code"] == "ALREADY_ONBOARDED"


def test_onboard_unsupported_country(client: TestClient) -> None:
    org = str(uuid.uuid4())
    r = client.post(
        "/v1/merchants/onboard",
        headers=auth_headers(org),
        json={"org_id": org, "business_name": "X", "country": "US", "type": "company"},
    )
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "UNSUPPORTED_COUNTRY"


def test_onboard_non_member_is_org_not_found(client: TestClient) -> None:
    # Authenticated for one org, onboarding a different org → ORG_NOT_FOUND (no leak).
    headers = auth_headers(str(uuid.uuid4()))
    r = client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={
            "org_id": str(uuid.uuid4()),
            "business_name": "X",
            "country": "KE",
            "type": "company",
        },
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "ORG_NOT_FOUND"


# ── get ──
def test_get_merchant_full_record_hides_bank_ref(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    secret = "254700999888"
    client.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={"rail": "mpesa", "account_details": secret, "currency": "KES", "country": "KE"},
    )
    r = client.get(f"/v1/merchants/{mid}", headers=headers)
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "PENDING_VERIFICATION"
    assert len(body["bank_accounts"]) == 1
    ba = body["bank_accounts"][0]
    assert ba["status"] == "PENDING_VERIFY"
    # No ref / plaintext leaks in the record.
    assert "account_ref" not in ba and "account_details" not in ba
    assert secret not in r.text


def test_get_unknown_merchant_404(client: TestClient) -> None:
    r = client.get(f"/v1/merchants/{uuid.uuid4()}", headers=auth_headers(str(uuid.uuid4())))
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "MERCHANT_NOT_FOUND"


# ── documents ──
def test_upload_document(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    r = client.post(
        f"/v1/merchants/{mid}/documents",
        headers=headers,
        files={"file": ("cert.pdf", b"%PDF-1.4 fake", "application/pdf")},
        data={"kind": "cert_incorporation"},
    )
    assert r.status_code == 201, r.text
    assert r.json()["status"] == "UPLOADED"


def test_upload_document_too_large(client: TestClient, settings) -> None:  # type: ignore[no-untyped-def]
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    oversize = b"x" * (settings.max_document_bytes + 1)
    r = client.post(
        f"/v1/merchants/{mid}/documents",
        headers=headers,
        files={"file": ("big.bin", oversize, "application/octet-stream")},
        data={"kind": "other"},
    )
    assert r.status_code == 413
    assert r.json()["error"]["code"] == "PAYLOAD_TOO_LARGE"


# ── bank accounts ──
def test_add_bank_account(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    r = client.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={
            "rail": "mpesa",
            "account_details": "254700123456",
            "currency": "KES",
            "country": "KE",
        },
    )
    assert r.status_code == 201, r.text
    body = r.json()
    assert body["status"] == "PENDING_VERIFY"
    # The response never echoes the secret.
    assert "account_details" not in body and "account_ref" not in body


def test_add_bank_account_invalid_rail_is_422_via_validation(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    r = client.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={"rail": "paypal", "account_details": "x", "currency": "USD", "country": "KE"},
    )
    # Unknown enum value → schema validation rejects with INVALID_PAYLOAD (400).
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_verify_bank_account_then_conflict(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    add = client.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={
            "rail": "mpesa",
            "account_details": "254700123456",
            "currency": "KES",
            "country": "KE",
        },
    )
    bid = add.json()["bank_account_id"]
    ok = client.post(f"/v1/merchants/{mid}/bank-accounts/{bid}/verify", headers=headers, json={})
    assert ok.status_code == 200 and ok.json()["status"] == "VERIFIED"
    again = client.post(f"/v1/merchants/{mid}/bank-accounts/{bid}/verify", headers=headers, json={})
    assert again.status_code == 409
    assert again.json()["error"]["code"] == "INVALID_TRANSITION"


# ── contracts ──
def test_accept_and_list_contracts(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    accepted = client.post(
        f"/v1/merchants/{mid}/contracts",
        headers=headers,
        json={"contract_version": "v1", "accepted": True},
    )
    assert accepted.status_code == 201, accepted.text
    listed = client.get(f"/v1/merchants/{mid}/contracts", headers=headers)
    assert listed.status_code == 200
    assert len(listed.json()["items"]) == 1


# ── fee tier ──
def test_fee_tier_get_and_admin_patch(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org, role="owner")
    mid = _onboard(client, headers, org)
    got = client.get(f"/v1/merchants/{mid}/fee-tier", headers=headers)
    assert got.status_code == 200 and got.json()["tier"] == "standard"
    patched = client.patch(
        f"/v1/merchants/{mid}/fee-tier", headers=headers, json={"tier": "enterprise"}
    )
    assert patched.status_code == 200 and patched.json()["tier"] == "enterprise"


def test_fee_tier_patch_forbidden_for_non_admin(client: TestClient) -> None:
    org = str(uuid.uuid4())
    owner = auth_headers(org, role="owner")
    mid = _onboard(client, owner, org)
    viewer = auth_headers(org, role="viewer")
    r = client.patch(f"/v1/merchants/{mid}/fee-tier", headers=viewer, json={"tier": "enterprise"})
    assert r.status_code == 403
    assert r.json()["error"]["code"] == "FORBIDDEN"


# ── internal decision → ACTIVE ──
def test_internal_decision_drives_state_machine(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    # Meet activation preconditions: verified bank + accepted contract.
    add = client.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={
            "rail": "mpesa",
            "account_details": "254700123456",
            "currency": "KES",
            "country": "KE",
        },
    )
    bid = add.json()["bank_account_id"]
    client.post(f"/v1/merchants/{mid}/bank-accounts/{bid}/verify", headers=headers, json={})
    client.post(
        f"/v1/merchants/{mid}/contracts",
        headers=headers,
        json={"contract_version": "v1", "accepted": True},
    )
    # Internal decision endpoint is NOT JWT-gated (no Authorization header).
    decided = client.post(f"/internal/merchants/{mid}/decision", json={"decision": "approve"})
    assert decided.status_code == 200, decided.text
    assert decided.json()["status"] == "ACTIVE"
    # And the merchant now reads ACTIVE.
    assert client.get(f"/v1/merchants/{mid}", headers=headers).json()["status"] == "ACTIVE"


def test_internal_decision_blocked_without_preconditions(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = auth_headers(org)
    mid = _onboard(client, headers, org)
    r = client.post(f"/internal/merchants/{mid}/decision", json={"decision": "approve"})
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "INVALID_TRANSITION"


# ── idempotency ──
def test_idempotent_onboard_replay(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = {**auth_headers(org), "Idempotency-Key": "onb-1"}
    body = {"org_id": org, "business_name": "Acme", "country": "KE", "type": "company"}
    first = client.post("/v1/merchants/onboard", headers=headers, json=body)
    second = client.post("/v1/merchants/onboard", headers=headers, json=body)
    assert first.status_code == 201 and second.status_code == 201
    assert first.json() == second.json()  # replay, not ALREADY_ONBOARDED


def test_idempotent_conflict_on_body_change(client: TestClient) -> None:
    org = str(uuid.uuid4())
    headers = {**auth_headers(org), "Idempotency-Key": "onb-2"}
    client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={"org_id": org, "business_name": "Acme", "country": "KE", "type": "company"},
    )
    r = client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={"org_id": org, "business_name": "Other", "country": "KE", "type": "company"},
    )
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "IDEMPOTENT_CONFLICT"
