"""End-to-end flows over REAL Postgres + Redis (exercises the real RefundRepository SQL).

The refund flow needs payment-orchestrator / paylink-service, which aren't running here, so we swap
just those two upstream clients on app.state for in-memory fakes; everything else (the repo, the
outbox, idempotency, HMAC) runs for real against Postgres/Redis.
"""

from __future__ import annotations

import json

import pytest
from fastapi.testclient import TestClient

from app.security.hmac import compute_signature
from tests._support import WEBHOOK_SECRET, FakePaylinksClient, FakePaymentsClient, auth_headers

pytestmark = pytest.mark.integration


def test_full_refund_lifecycle(live_client: TestClient) -> None:
    payments = FakePaymentsClient()
    payments.add("pay-int", paylink_id="0xpl-int", rail="mpesa", status="SETTLED")
    paylinks = FakePaylinksClient()
    paylinks.add("0xpl-int", amount_minor=1000)
    live_client.app.state.payments_client = payments
    live_client.app.state.paylinks_client = paylinks

    admin = auth_headers(user_roles=["admin"])
    created = live_client.post(
        "/v1/refunds", headers=admin, json={"payment_id": "pay-int", "amount_minor": 1000}
    )
    assert created.status_code == 201, created.text
    rid = created.json()["refund_id"]
    assert created.json()["is_partial"] is False

    approved = live_client.post(f"/v1/refunds/{rid}/approve", headers=admin)
    assert approved.status_code == 200
    assert approved.json()["state"] == "PROCESSING"

    got = live_client.get(f"/v1/refunds/{rid}", headers=admin)
    assert got.status_code == 200
    listed = live_client.get("/v1/refunds", headers=admin, params={"payment_id": "pay-int"})
    assert len(listed.json()["refunds"]) == 1


def test_full_dispute_lifecycle(live_client: TestClient) -> None:
    def webhook(body: dict) -> dict:
        raw = json.dumps(body).encode()
        r = live_client.post(
            "/v1/disputes/webhooks/stub",
            content=raw,
            headers={"X-Signature": compute_signature(WEBHOOK_SECRET, raw)},
        )
        assert r.status_code == 200, r.text
        return r.json()

    opened = webhook(
        {
            "kind": "dispute.opened",
            "provider_dispute_id": "int-d1",
            "payment_id": "pay-int",
            "rail": "card",
            "amount_minor": 500,
            "currency": "KES",
        }
    )
    did = opened["dispute_id"]
    admin = auth_headers(user_roles=["admin"])
    ev = live_client.post(
        f"/v1/disputes/{did}/evidence", headers=admin, json={"kind": "receipt", "summary": "x"}
    )
    assert ev.status_code == 201
    sub = live_client.post(f"/v1/disputes/{did}/submit", headers=admin)
    assert sub.status_code == 200
    res = webhook({"kind": "dispute.resolved", "provider_dispute_id": "int-d1", "outcome": "lost"})
    assert res["action"] == "resolved"
    got = live_client.get(f"/v1/disputes/{did}", headers=admin)
    assert got.json()["state"] == "LOST"
    assert got.json()["clawback_requested"] is True
