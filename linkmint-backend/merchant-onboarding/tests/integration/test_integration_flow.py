"""End-to-end flow against REAL Postgres + Redis — the acceptance round-trip + invariants."""

from __future__ import annotations

import uuid

import pytest
import sqlalchemy as sa
from fastapi.testclient import TestClient

from tests._support import auth_headers_for, mint_token

pytestmark = pytest.mark.integration


def _onboard(client: TestClient, headers: dict[str, str], org_id: str) -> str:
    r = client.post(
        "/v1/merchants/onboard",
        headers=headers,
        json={
            "org_id": org_id,
            "business_name": "Acme Ltd",
            "registration_no": "C.999",
            "country": "KE",
            "type": "company",
        },
    )
    assert r.status_code == 201, r.text
    return r.json()["merchant_id"]


def test_full_acceptance_roundtrip(live_client: TestClient, pg_url: str) -> None:
    c = live_client
    org = str(uuid.uuid4())
    headers = auth_headers_for(org, role="owner")
    secret = "254700ABCDEF"  # the plaintext bank ref — must never hit the DB

    mid = _onboard(c, headers, org)

    # Approving before preconditions are met → INVALID_TRANSITION (409).
    early = c.post(f"/internal/merchants/{mid}/decision", json={"decision": "approve"})
    assert early.status_code == 409
    assert early.json()["error"]["code"] == "INVALID_TRANSITION"

    # Upload a document (multipart).
    doc = c.post(
        f"/v1/merchants/{mid}/documents",
        headers=headers,
        files={"file": ("cert.pdf", b"%PDF-1.4 fake", "application/pdf")},
        data={"kind": "cert_incorporation"},
    )
    assert doc.status_code == 201 and doc.json()["status"] == "UPLOADED"

    # Add + verify a bank account.
    add = c.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={"rail": "mpesa", "account_details": secret, "currency": "KES", "country": "KE"},
    )
    assert add.status_code == 201 and add.json()["status"] == "PENDING_VERIFY"
    bid = add.json()["bank_account_id"]
    verified = c.post(f"/v1/merchants/{mid}/bank-accounts/{bid}/verify", headers=headers, json={})
    assert verified.status_code == 200 and verified.json()["status"] == "VERIFIED"

    # Accept a contract.
    accepted = c.post(
        f"/v1/merchants/{mid}/contracts",
        headers=headers,
        json={"contract_version": "v1", "accepted": True},
    )
    assert accepted.status_code == 201

    # Now the decision drives PENDING_VERIFICATION → ACTIVE.
    decided = c.post(f"/internal/merchants/{mid}/decision", json={"decision": "approve"})
    assert decided.status_code == 200 and decided.json()["status"] == "ACTIVE"

    got = c.get(f"/v1/merchants/{mid}", headers=headers)
    assert got.status_code == 200 and got.json()["status"] == "ACTIVE"
    # Bank account surfaced by status only; the plaintext ref never appears in the response.
    assert secret not in got.text
    assert all("account_ref" not in ba for ba in got.json()["bank_accounts"])

    # INVARIANT (DB): the persisted account_ref is AES-GCM ciphertext, NOT the plaintext.
    engine = sa.create_engine(pg_url)
    with engine.connect() as conn:
        refs = list(
            conn.execute(sa.text("SELECT account_ref FROM merchant.bank_accounts")).scalars()
        )
        assert refs and all(secret not in r for r in refs)

        # The outbox recorded the lifecycle events (no plaintext in any payload).
        kinds = list(conn.execute(sa.text("SELECT kind FROM merchant.merchant_events")).scalars())
        assert "merchant.onboarded" in kinds
        assert "merchant.bank_account.added" in kinds
        assert "merchant.bank_account.verified" in kinds
        assert "merchant.contract.accepted" in kinds
        assert "merchant.verified" in kinds
        payload_blob = " ".join(
            str(p)
            for p in conn.execute(
                sa.text("SELECT payload::text FROM merchant.merchant_events")
            ).scalars()
        )
        assert secret not in payload_blob
    engine.dispose()


def test_consumer_seam_drives_decision_through_real_db(
    live_client: TestClient, pg_url: str
) -> None:
    """The compliance.kyb.* / admin.override.* consumer writes through to the real DB."""
    import asyncio

    from app.db.repositories import MerchantRepository
    from app.db.session import make_engine, make_sessionmaker
    from app.domain.merchants_service import MerchantsService
    from app.events.consumer import KYB_PASSED, MerchantEventConsumer
    from app.events.stub import NoopPublisher
    from tests._support import make_settings

    c = live_client
    org = str(uuid.uuid4())
    headers = auth_headers_for(org, role="owner")
    mid = _onboard(c, headers, org)
    # Meet preconditions through the HTTP surface.
    add = c.post(
        f"/v1/merchants/{mid}/bank-accounts",
        headers=headers,
        json={
            "rail": "mpesa",
            "account_details": "254700111222",
            "currency": "KES",
            "country": "KE",
        },
    )
    bid = add.json()["bank_account_id"]
    c.post(f"/v1/merchants/{mid}/bank-accounts/{bid}/verify", headers=headers, json={})
    c.post(
        f"/v1/merchants/{mid}/contracts",
        headers=headers,
        json={"contract_version": "v1", "accepted": True},
    )

    settings = make_settings(database_url=pg_url)

    async def _drive() -> str:
        engine = make_engine(pg_url)
        sm = make_sessionmaker(engine)
        try:
            async with sm() as session:
                merchants = MerchantsService(
                    MerchantRepository(session), session.commit, NoopPublisher(), settings
                )
                consumer = MerchantEventConsumer(merchants)
                await consumer.handle(KYB_PASSED, {"merchant_id": mid})
                fetched = await MerchantRepository(session).get_merchant(uuid.UUID(mid))
                return fetched.status if fetched else "MISSING"
        finally:
            await engine.dispose()

    assert asyncio.run(_drive()) == "ACTIVE"
    assert c.get(f"/v1/merchants/{mid}", headers=headers).json()["status"] == "ACTIVE"


def test_token_with_wrong_org_cannot_see_merchant(live_client: TestClient) -> None:
    c = live_client
    org = str(uuid.uuid4())
    mid = _onboard(c, auth_headers_for(org, role="owner"), org)
    # A valid token for a DIFFERENT org → 404 (no existence leak).
    other = {"Authorization": f"Bearer {mint_token(roles=[(str(uuid.uuid4()), 'owner')])}"}
    r = c.get(f"/v1/merchants/{mid}", headers=other)
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "MERCHANT_NOT_FOUND"
