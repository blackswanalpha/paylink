"""GET merchant pricing — both routes, org-member/admin authz, no-leak."""

from __future__ import annotations

import uuid

from fastapi.testclient import TestClient

from tests._support import FakePricingRepository, auth_headers


def test_both_routes_return_same_body(client: TestClient, fake_repo: FakePricingRepository) -> None:
    mid, org = uuid.uuid4(), uuid.uuid4()
    fake_repo.seed_merchant(mid, tier="growth", org_id=org)
    headers = auth_headers(user_roles=["admin"])
    a = client.get(f"/v1/pricing/merchants/{mid}", headers=headers)
    b = client.get(f"/v1/merchants/{mid}/pricing", headers=headers)
    assert a.status_code == 200 and b.status_code == 200
    assert a.json() == b.json()
    assert a.json()["tier"] == "growth"
    assert a.json()["org_id"] == str(org)
    # rail fees for the tier are included.
    assert {rf["rail"] for rf in a.json()["rail_fees"]} == {"mpesa", "card", "bank", "crypto"}


def test_org_member_can_read_own(client: TestClient, fake_repo: FakePricingRepository) -> None:
    mid, org = uuid.uuid4(), uuid.uuid4()
    fake_repo.seed_merchant(mid, org_id=org)
    headers = auth_headers(roles=[{"org_id": str(org), "role": "admin"}], user_roles=["merchant"])
    r = client.get(f"/v1/pricing/merchants/{mid}", headers=headers)
    assert r.status_code == 200


def test_non_member_gets_not_found_no_leak(
    client: TestClient, fake_repo: FakePricingRepository
) -> None:
    mid, org = uuid.uuid4(), uuid.uuid4()
    fake_repo.seed_merchant(mid, org_id=org)
    # A different org's member — must not learn the merchant exists.
    headers = auth_headers(
        roles=[{"org_id": str(uuid.uuid4()), "role": "admin"}], user_roles=["merchant"]
    )
    r = client.get(f"/v1/pricing/merchants/{mid}", headers=headers)
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "MERCHANT_PRICING_NOT_FOUND"


def test_unknown_merchant_404(client: TestClient) -> None:
    r = client.get(
        f"/v1/pricing/merchants/{uuid.uuid4()}", headers=auth_headers(user_roles=["admin"])
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "MERCHANT_PRICING_NOT_FOUND"


def test_invalid_uuid_400(client: TestClient) -> None:
    r = client.get("/v1/pricing/merchants/not-a-uuid", headers=auth_headers(user_roles=["admin"]))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"
