"""End-to-end Flow E (compliance block) against real Postgres + Redis.

create KYC session → signed provider callback (grants a tier) → seed prior activity → call the
internal ``/v1/risk/evaluate`` and assert a tier-1 user is BLOCKED by the Kenya AML threshold, with
the ``risk_scores`` row, the ``block`` flag, and the ``compliance_events`` outbox rows all persisted.
"""

from __future__ import annotations

import json
import uuid

import pytest
import sqlalchemy as sa
from fastapi.testclient import TestClient

from app.security.hmac import compute_signature
from tests._support import CALLBACK_SECRET, user_headers

pytestmark = pytest.mark.integration


def _sync_url(url: str) -> str:
    return url.replace("+psycopg", "")  # psycopg3 works for sync SQLAlchemy too via the same driver


def _seed_activity(pg_url: str, user_id: uuid.UUID, amount: str) -> None:
    engine = sa.create_engine(pg_url)
    with engine.begin() as conn:
        conn.execute(
            sa.text(
                "INSERT INTO compliance.activity_events (user_id, action, amount, currency) "
                "VALUES (:uid, 'payment.initiated', :amt, 'KES')"
            ),
            {"uid": str(user_id), "amt": amount},
        )
    engine.dispose()


def test_kyc_pass_then_aml_block_flow(live_client: TestClient, pg_url: str) -> None:
    # ── 1. KYC: create a session for user A and pass them to tier 2 via a signed callback. ──
    user_a = str(uuid.uuid4())
    sess = live_client.post(
        "/v1/kyc/sessions",
        json={"user_id": user_a, "tier_requested": 2},
        headers=user_headers(user_a),
    )
    assert sess.status_code == 201
    assert sess.json()["provider_url"].startswith("https://kyc.stub.local/s/")

    raw = json.dumps({"user_id": user_a, "status": "passed", "tier": 2}).encode()
    sig = compute_signature(CALLBACK_SECRET, raw)
    cb = live_client.post("/v1/kyc/callbacks/stub", content=raw, headers={"X-Signature": sig})
    assert cb.status_code == 200 and cb.json() == {"ok": True}

    status = live_client.get(
        "/v1/compliance/status", params={"user_id": user_a}, headers=user_headers(user_a)
    )
    assert status.status_code == 200 and status.json()["kyc_tier"] == 2

    # ── 2. Risk: a TIER-1 user with prior activity that crosses the AML threshold is blocked. ──
    user_b = uuid.uuid4()
    raw_b = json.dumps({"user_id": str(user_b), "status": "passed", "tier": 1}).encode()
    sig_b = compute_signature(CALLBACK_SECRET, raw_b)
    assert (
        live_client.post(
            "/v1/kyc/callbacks/stub", content=raw_b, headers={"X-Signature": sig_b}
        ).status_code
        == 200
    )

    _seed_activity(pg_url, user_b, "120000")  # prior cumulative

    resp = live_client.post(
        "/v1/risk/evaluate",
        json={
            "user_id": str(user_b),
            "action": "paylink.create",
            "amount": 40000,  # 120k + 40k = 160k ≥ 150k, tier 1 → block
            "currency": "KES",
            "geo": "NG",
            "registered_country": "KE",
        },
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["decision"] == "block"
    assert any(r["code"] == "AML_THRESHOLD" for r in body["reasons"])

    # ── 3. Assert the persisted rows (risk_score + block flag + outbox events). ──
    engine = sa.create_engine(pg_url)
    with engine.connect() as conn:
        score_count = conn.execute(
            sa.text("SELECT count(*) FROM compliance.risk_scores WHERE user_id = :u"),
            {"u": str(user_b)},
        ).scalar_one()
        assert score_count == 1

        flag = conn.execute(
            sa.text(
                "SELECT severity, kind FROM compliance.flags WHERE user_id = :u ORDER BY id DESC"
            ),
            {"u": str(user_b)},
        ).first()
        assert flag is not None and flag.severity == "block"

        event_kinds = set(
            conn.execute(
                sa.text(
                    "SELECT kind FROM compliance.compliance_events "
                    "WHERE subject_id = :u OR payload->>'user_id' = :us"
                ),
                {"u": str(user_b), "us": str(user_b)},
            ).scalars()
        )
        assert "compliance.check.failed" in event_kinds
        assert "compliance.flag.raised" in event_kinds
        # The tier-1 KYC pass for user_b also wrote a kyc.passed event.
        assert "compliance.kyc.passed" in event_kinds
    engine.dispose()
