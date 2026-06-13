"""HTTP surface for /v1/disputes — HMAC webhook + evidence/submit/read."""

from __future__ import annotations

import json

from fastapi.testclient import TestClient

from app.security.hmac import compute_signature
from tests._support import WEBHOOK_SECRET, auth_headers


def _post_webhook(
    client: TestClient, body: dict, *, provider: str = "stub", secret: str | None = None
):
    raw = json.dumps(body).encode()
    sig = compute_signature(secret if secret is not None else WEBHOOK_SECRET, raw)
    return client.post(
        f"/v1/disputes/webhooks/{provider}",
        content=raw,
        headers={"X-Signature": sig, "Content-Type": "application/json"},
    )


def _open(client: TestClient, pid: str = "dp-1") -> str:
    r = _post_webhook(
        client,
        {
            "kind": "dispute.opened",
            "provider_dispute_id": pid,
            "payment_id": "pay-1",
            "rail": "card",
            "amount_minor": 1000,
            "currency": "KES",
        },
    )
    assert r.status_code == 200, r.text
    assert r.json()["action"] == "opened"
    return r.json()["dispute_id"]


def test_webhook_opens_dispute(client: TestClient) -> None:
    did = _open(client)
    assert did


def test_webhook_unknown_provider_404(client: TestClient) -> None:
    r = _post_webhook(
        client, {"kind": "dispute.opened", "provider_dispute_id": "x"}, provider="nope"
    )
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "UNKNOWN_PROVIDER"


def test_webhook_bad_signature_401(client: TestClient) -> None:
    r = _post_webhook(
        client, {"kind": "dispute.opened", "provider_dispute_id": "x"}, secret="wrong"
    )
    assert r.status_code == 401
    assert r.json()["error"]["code"] == "INVALID_SIGNATURE"


def test_webhook_replay_is_noop(client: TestClient) -> None:
    _open(client, pid="dup")
    r = _post_webhook(
        client, {"kind": "dispute.opened", "provider_dispute_id": "dup", "rail": "card"}
    )
    assert r.status_code == 200
    assert r.json()["action"] == "opened_replay"


def test_webhook_resolution_flow(client: TestClient) -> None:
    _open(client, pid="res")
    r = _post_webhook(
        client, {"kind": "dispute.resolved", "provider_dispute_id": "res", "outcome": "lost"}
    )
    assert r.status_code == 200
    assert r.json()["action"] == "resolved"


def test_evidence_and_submit_and_get(client: TestClient) -> None:
    did = _open(client, pid="ev")
    admin = auth_headers(user_roles=["admin"])
    # add evidence
    r = client.post(
        f"/v1/disputes/{did}/evidence",
        headers=admin,
        json={"kind": "receipt", "summary": "delivered", "payload": {"url": "x"}},
    )
    assert r.status_code == 201, r.text
    # submit
    r = client.post(f"/v1/disputes/{did}/submit", headers=admin)
    assert r.status_code == 200
    assert r.json()["state"] == "SUBMITTED"
    # get includes the evidence
    r = client.get(f"/v1/disputes/{did}", headers=admin)
    assert r.status_code == 200
    body = r.json()
    assert len(body["evidence"]) == 1
    assert body["evidence"][0]["kind"] == "receipt"


def test_get_dispute_requires_membership(client: TestClient) -> None:
    did = _open(client, pid="scoped")
    # a non-admin, non-member caller cannot read an unscoped dispute
    r = client.get(f"/v1/disputes/{did}", headers=auth_headers())
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "ORG_NOT_FOUND"


def test_webhook_invalid_json_400(client: TestClient) -> None:
    raw = b"not json"
    sig = compute_signature(WEBHOOK_SECRET, raw)
    r = client.post(
        "/v1/disputes/webhooks/stub",
        content=raw,
        headers={"X-Signature": sig, "Content-Type": "application/json"},
    )
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"
