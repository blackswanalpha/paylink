"""End-to-end over REAL Postgres + Redis: quote → accrual → monthly invoice, plus merchant read."""

from __future__ import annotations

import uuid

import pytest
import sqlalchemy as sa
from fastapi.testclient import TestClient

from tests._support import auth_headers

pytestmark = pytest.mark.integration


def test_quote_accrual_invoice_flow(live_client: TestClient, pg_url: str) -> None:
    merchant_id = str(uuid.uuid4())

    # 1. Same-currency quote across two rails (persists quote rows + outbox events).
    r = live_client.post(
        "/v1/pricing/quote",
        json={
            "merchant_id": merchant_id,
            "gross": 100_000,
            "currency": "KES",
            "rails": ["mpesa", "card"],
            "tiers": ["standard"],
        },
        headers=auth_headers(),
    )
    assert r.status_code == 200, r.text
    assert len(r.json()["quotes"]) == 2

    # 2. Cross-currency quote — the real StaticFxProvider + Redis cache + locked rate.
    r = live_client.post(
        "/v1/pricing/quote",
        json={
            "merchant_id": merchant_id,
            "gross": 100,
            "currency": "USD",
            "settle_currency": "KES",
            "rails": ["mpesa"],
            "tiers": ["standard"],
        },
        headers=auth_headers(),
    )
    assert r.status_code == 200, r.text
    assert r.json()["quotes"][0]["gross_settled"] == 12_950
    assert r.json()["quotes"][0]["fx"]["rate"] == "129.50"

    # 3. Two realized platform-fee accruals for the same merchant + period.
    for i, amt in enumerate([2_500, 1_500]):
        a = live_client.post(
            "/v1/internal/accruals",
            json={
                "merchant_id": merchant_id,
                "amount": amt,
                "currency": "KES",
                "source_ref": f"pay-{i}",
                "occurred_at": "2026-05-15T12:00:00+00:00",
            },
        )
        assert a.status_code == 202, a.text

    # 4. Generate the monthly invoice → one invoice with the summed total.
    run = live_client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert run.status_code == 200, run.text
    gen = run.json()["generated"]
    assert len(gen) == 1
    assert gen[0]["total_fee"] == 4_000
    assert gen[0]["line_count"] == 2

    # 5. Re-run is idempotent — no second invoice.
    run2 = live_client.post("/v1/internal/invoices/platform-fee/run", json={"period": "2026-05"})
    assert run2.json()["generated"] == []

    # Verify the durable state directly in Postgres.
    engine = sa.create_engine(pg_url)
    with engine.connect() as conn:
        inv_count = conn.execute(
            sa.text("SELECT count(*) FROM pricing.platform_fee_invoices WHERE merchant_id = :m"),
            {"m": uuid.UUID(merchant_id)},
        ).scalar_one()
        assert inv_count == 1
        unbilled = conn.execute(
            sa.text(
                "SELECT count(*) FROM pricing.platform_fee_accruals "
                "WHERE merchant_id = :m AND invoice_id IS NULL"
            ),
            {"m": uuid.UUID(merchant_id)},
        ).scalar_one()
        assert unbilled == 0  # both accruals stamped
        quote_count = conn.execute(
            sa.text("SELECT count(*) FROM pricing.quotes WHERE merchant_id = :m"),
            {"m": uuid.UUID(merchant_id)},
        ).scalar_one()
        assert quote_count == 3  # 2 same-currency + 1 cross-currency
    engine.dispose()


def test_merchant_pricing_read_over_real_db(live_client: TestClient, pg_url: str) -> None:
    merchant_id, org_id = uuid.uuid4(), uuid.uuid4()
    engine = sa.create_engine(pg_url)
    with engine.begin() as conn:
        conn.execute(
            sa.text(
                "INSERT INTO pricing.merchant_pricing "
                "(merchant_id, org_id, tier, source, effective_at, updated_at) "
                "VALUES (:m, :o, 'growth', 'manual', now(), now())"
            ),
            {"m": merchant_id, "o": org_id},
        )
    engine.dispose()

    r = live_client.get(
        f"/v1/pricing/merchants/{merchant_id}", headers=auth_headers(user_roles=["admin"])
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["tier"] == "growth"
    assert body["org_id"] == str(org_id)
    assert {rf["rail"] for rf in body["rail_fees"]} == {"mpesa", "card", "bank", "crypto"}
