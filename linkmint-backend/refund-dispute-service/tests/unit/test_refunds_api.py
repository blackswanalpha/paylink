"""HTTP surface for /v1/refunds — auth, RBAC, idempotency, error envelope."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import FakePaymentsClient, auth_headers


def _seed(fake_payments: FakePaymentsClient, status: str = "SETTLED") -> None:
    fake_payments.add("pay1", paylink_id="0xpl", rail="mpesa", status=status)


def test_create_refund_unscoped(client: TestClient, fake_payments: FakePaymentsClient) -> None:
    _seed(fake_payments)
    r = client.post(
        "/v1/refunds",
        headers=auth_headers(),
        json={"payment_id": "pay1", "amount_minor": 500, "currency": "KES"},
    )
    assert r.status_code == 201, r.text
    body = r.json()
    assert body["state"] == "REQUESTED"
    assert body["rail"] == "mpesa"


def test_create_refund_missing_auth_401(client: TestClient) -> None:
    r = client.post("/v1/refunds", json={"payment_id": "p", "amount_minor": 1})
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "UNAUTHORIZED"


def test_create_refund_bad_token_401(client: TestClient) -> None:
    r = client.post(
        "/v1/refunds",
        headers={"Authorization": "Bearer not.a.jwt"},
        json={"payment_id": "p", "amount_minor": 1},
    )
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_TOKEN"


def test_create_refund_payment_not_found(client: TestClient) -> None:
    r = client.post(
        "/v1/refunds", headers=auth_headers(), json={"payment_id": "ghost", "amount_minor": 1}
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "PAYMENT_NOT_FOUND"


def test_create_refund_org_requires_membership(
    client: TestClient, fake_payments: FakePaymentsClient
) -> None:
    _seed(fake_payments)
    org = str(uuid.uuid4())
    # caller is not a member of `org`
    r = client.post(
        "/v1/refunds",
        headers=auth_headers(),
        json={"payment_id": "pay1", "amount_minor": 100, "org_id": org},
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "ORG_NOT_FOUND"


def test_create_refund_org_member_ok(client: TestClient, fake_payments: FakePaymentsClient) -> None:
    _seed(fake_payments)
    org = str(uuid.uuid4())
    r = client.post(
        "/v1/refunds",
        headers=auth_headers(roles=[{"org_id": org, "role": "member"}]),
        json={"payment_id": "pay1", "amount_minor": 100, "org_id": org},
    )
    assert r.status_code == 201


def test_approve_requires_admin(client: TestClient, fake_payments: FakePaymentsClient) -> None:
    _seed(fake_payments)
    org = str(uuid.uuid4())
    member = auth_headers(roles=[{"org_id": org, "role": "member"}])
    created = client.post(
        "/v1/refunds",
        headers=member,
        json={"payment_id": "pay1", "amount_minor": 100, "org_id": org},
    ).json()
    rid = created["refund_id"]
    # member cannot approve
    r = client.post(f"/v1/refunds/{rid}/approve", headers=member)
    assert r.status_code == 403
    # owner can
    owner = auth_headers(roles=[{"org_id": org, "role": "owner"}])
    r = client.post(f"/v1/refunds/{rid}/approve", headers=owner)
    assert r.status_code == 200
    assert r.json()["state"] == "PROCESSING"


def test_approve_unscoped_by_platform_admin(
    client: TestClient, fake_payments: FakePaymentsClient
) -> None:
    _seed(fake_payments)
    created = client.post(
        "/v1/refunds", headers=auth_headers(), json={"payment_id": "pay1", "amount_minor": 100}
    ).json()
    rid = created["refund_id"]
    admin = auth_headers(user_roles=["admin"])
    r = client.post(f"/v1/refunds/{rid}/approve", headers=admin)
    assert r.status_code == 200


def test_reject(client: TestClient, fake_payments: FakePaymentsClient) -> None:
    _seed(fake_payments)
    created = client.post(
        "/v1/refunds",
        headers=auth_headers(user_roles=["admin"]),
        json={"payment_id": "pay1", "amount_minor": 100},
    ).json()
    rid = created["refund_id"]
    r = client.post(f"/v1/refunds/{rid}/reject", headers=auth_headers(user_roles=["admin"]))
    assert r.status_code == 200
    assert r.json()["state"] == "REJECTED"


def test_get_and_list(client: TestClient, fake_payments: FakePaymentsClient) -> None:
    _seed(fake_payments)
    admin = auth_headers(user_roles=["admin"])
    created = client.post(
        "/v1/refunds", headers=admin, json={"payment_id": "pay1", "amount_minor": 100}
    ).json()
    rid = created["refund_id"]
    r = client.get(f"/v1/refunds/{rid}", headers=admin)
    assert r.status_code == 200
    r = client.get("/v1/refunds", headers=admin, params={"payment_id": "pay1"})
    assert r.status_code == 200
    assert len(r.json()["refunds"]) == 1


def test_get_unknown_404(client: TestClient) -> None:
    r = client.get(f"/v1/refunds/{uuid.uuid4()}", headers=auth_headers(user_roles=["admin"]))
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "REFUND_NOT_FOUND"


def test_invalid_refund_id_400(client: TestClient) -> None:
    r = client.get("/v1/refunds/not-a-uuid", headers=auth_headers(user_roles=["admin"]))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_idempotency_replay_and_conflict(
    client: TestClient, fake_payments: FakePaymentsClient
) -> None:
    _seed(fake_payments)
    headers = {**auth_headers(user_roles=["admin"]), "Idempotency-Key": "k-1"}
    body = {"payment_id": "pay1", "amount_minor": 100}
    r1 = client.post("/v1/refunds", headers=headers, json=body)
    assert r1.status_code == 201
    # same key + same body → replayed (same refund_id)
    r2 = client.post("/v1/refunds", headers=headers, json=body)
    assert r2.status_code == 201
    assert r2.json()["refund_id"] == r1.json()["refund_id"]
    # same key + different body → 409 conflict
    r3 = client.post(
        "/v1/refunds", headers=headers, json={"payment_id": "pay1", "amount_minor": 200}
    )
    assert r3.status_code == 409
    assert r3.json()["error"]["code"] == "IDEMPOTENT_CONFLICT"
