"""POST /v1/pricing/quote — breakdown, FX lock, idempotency, auth."""

from __future__ import annotations

import uuid
from decimal import Decimal

from fastapi.testclient import TestClient

from tests._support import FakePricingRepository, auth_headers, quote_body


def test_quote_returns_breakdown(client: TestClient, fake_repo: FakePricingRepository) -> None:
    body = quote_body(gross=100_000, currency="KES", rails=["mpesa"], tiers=["standard"])
    r = client.post("/v1/pricing/quote", json=body, headers=auth_headers())
    assert r.status_code == 200
    quotes = r.json()["quotes"]
    assert len(quotes) == 1
    q = quotes[0]
    assert q["tier"] == "standard" and q["rail"] == "mpesa"
    assert q["platform_fee"] == 2_500 and q["rail_fee"] == 1_500 and q["net"] == 96_000
    assert q["fx"] is None
    assert "pricing.fee_quote.issued" in fake_repo.event_kinds()
    assert len(fake_repo.quotes) == 1


def test_quote_multiple_pairs(client: TestClient, fake_repo: FakePricingRepository) -> None:
    body = quote_body(rails=["mpesa", "card"], tiers=["standard", "enterprise"])
    r = client.post("/v1/pricing/quote", json=body, headers=auth_headers())
    assert r.status_code == 200
    assert len(r.json()["quotes"]) == 4  # 2 tiers × 2 rails
    assert len(fake_repo.quotes) == 4


def test_quote_locks_fx_rate(client: TestClient, fake_repo: FakePricingRepository) -> None:
    body = quote_body(
        gross=100, currency="USD", settle_currency="KES", rails=["mpesa"], tiers=["standard"]
    )
    r = client.post("/v1/pricing/quote", json=body, headers=auth_headers())
    assert r.status_code == 200
    q = r.json()["quotes"][0]
    assert q["gross_settled"] == 12_950
    assert q["fx"]["rate"] == "129.50"
    # The rate used == the rate persisted on the quote row (lock-at-quote).
    row = fake_repo.quotes[0]
    assert row.fx_rate == Decimal("129.50")
    assert row.fx_base == "USD" and row.fx_quote == "KES"


def test_quote_defaults_to_merchant_tier(
    client: TestClient, fake_repo: FakePricingRepository
) -> None:
    mid = uuid.uuid4()
    fake_repo.seed_merchant(mid, tier="enterprise")
    body = quote_body(merchant_id=str(mid), rails=["mpesa"])
    body.pop("tiers")
    r = client.post("/v1/pricing/quote", json=body, headers=auth_headers())
    assert r.status_code == 200
    assert r.json()["quotes"][0]["tier"] == "enterprise"


def test_quote_requires_auth(client: TestClient) -> None:
    assert client.post("/v1/pricing/quote", json=quote_body()).status_code == 401


def test_quote_idempotent_replays(client: TestClient, fake_repo: FakePricingRepository) -> None:
    headers = auth_headers()
    headers["Idempotency-Key"] = "quote-key-1"
    body = quote_body()
    r1 = client.post("/v1/pricing/quote", json=body, headers=headers)
    r2 = client.post("/v1/pricing/quote", json=body, headers=headers)
    assert r1.status_code == 200 and r2.status_code == 200
    assert r1.json() == r2.json()
    assert len(fake_repo.quotes) == 1  # replayed, not recomputed


def test_quote_unknown_tier_404(client: TestClient) -> None:
    body = quote_body(tiers=["does-not-exist"])
    r = client.post("/v1/pricing/quote", json=body, headers=auth_headers())
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "TIER_NOT_FOUND"
