"""API-level tests via TestClient with DB/Redis/chain replaced by in-memory fakes (no Docker)."""

from __future__ import annotations

from typing import Any

RECEIVER = "0x0000000000000000000000000000000000000004"
FUTURE = "2030-01-01T00:00:00Z"


def _body(**over: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "receiver": RECEIVER,
        "amount": 1500,
        "currency": "PLN",
        "expiry": FUTURE,
        "usage": "single",
        "metadata": {"orderId": "INV1"},
    }
    body.update(over)
    return body


def test_healthz(client: Any) -> None:
    r = client.get("/internal/healthz")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_metrics_exposed(client: Any) -> None:
    r = client.get("/metrics")
    assert r.status_code == 200
    assert "text/plain" in r.headers["content-type"]


def test_create_then_get(client: Any) -> None:
    r = client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "k1"})
    assert r.status_code == 201
    body = r.json()
    assert body["status"] == "PENDING"
    assert body["chain_tx_hash"] is not None
    assert r.headers.get("x-request-id")

    pl_id = body["pl_id"]
    g = client.get(f"/v1/paylinks/{pl_id}")
    assert g.status_code == 200
    assert g.json()["pl_id"] == pl_id
    assert g.json()["amount"] == 1500


def test_create_idempotent_replay(client: Any) -> None:
    headers = {"Idempotency-Key": "dup"}
    r1 = client.post("/v1/paylinks", json=_body(), headers=headers)
    r2 = client.post("/v1/paylinks", json=_body(), headers=headers)
    assert r1.status_code == r2.status_code == 201
    assert r1.json()["pl_id"] == r2.json()["pl_id"]


def test_idempotency_conflict_on_different_body(client: Any) -> None:
    headers = {"Idempotency-Key": "dup2"}
    client.post("/v1/paylinks", json=_body(amount=100), headers=headers)
    r = client.post("/v1/paylinks", json=_body(amount=200), headers=headers)
    assert r.status_code == 409
    assert r.json()["error"]["code"] == "IDEMPOTENT_CONFLICT"


def test_create_bad_amount_envelope(client: Any) -> None:
    r = client.post("/v1/paylinks", json=_body(amount=-1))
    assert r.status_code == 400
    err = r.json()["error"]
    assert err["code"] == "INVALID_PAYLOAD"
    assert set(err.keys()) == {"code", "message", "details", "trace_id"}


def test_create_bad_receiver(client: Any) -> None:
    r = client.post("/v1/paylinks", json=_body(receiver="not-an-address"))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"


def test_get_not_found(client: Any) -> None:
    r = client.get("/v1/paylinks/0xdeadbeef")
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "PAYLINK_NOT_FOUND"


def test_get_reflects_chain_read_through(client: Any, fake_chain: Any) -> None:
    pl_id = client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "v1"}).json()[
        "pl_id"
    ]
    fake_chain.paylinks[pl_id] = {
        "id": pl_id,
        "creator": "0x0",
        "receiver": RECEIVER,
        "owner": "0x0",
        "amount": 1500,
        "expiry": 4102444800,
        "status": "VERIFIED",
        "metadataHash": "0x0",
        "createdAt": 0,
        "voteCount": 3,
    }
    g = client.get(f"/v1/paylinks/{pl_id}")
    assert g.json()["status"] == "VERIFIED"
    assert g.json()["vote_count"] == 3


def test_list_pagination(client: Any) -> None:
    for i in range(3):
        client.post(
            "/v1/paylinks", json=_body(amount=100 + i), headers={"Idempotency-Key": f"k{i}"}
        )
    page1 = client.get("/v1/paylinks", params={"limit": 2}).json()
    assert len(page1["items"]) == 2
    assert page1["next_cursor"]
    page2 = client.get("/v1/paylinks", params={"limit": 2, "cursor": page1["next_cursor"]}).json()
    assert len(page2["items"]) == 1
    assert page2["next_cursor"] is None


def test_list_filter_by_creator(client: Any, signer: Any) -> None:
    client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "c1"})
    r = client.get("/v1/paylinks", params={"creator": signer.address})
    assert r.status_code == 200
    assert len(r.json()["items"]) >= 1


def test_list_invalid_status(client: Any) -> None:
    r = client.get("/v1/paylinks", params={"status": "BOGUS"})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_QUERY"


def test_cancel_flow(client: Any) -> None:
    pl_id = client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "cx"}).json()[
        "pl_id"
    ]
    r = client.post(f"/v1/paylinks/{pl_id}/cancel", headers={"Idempotency-Key": "cancelx"})
    assert r.status_code == 200
    assert r.json()["status"] == "CANCELLED"
    g = client.get(f"/v1/paylinks/{pl_id}")
    assert g.json()["status"] == "CANCELLED"


def test_cancel_not_found(client: Any) -> None:
    r = client.post("/v1/paylinks/0xmissing/cancel")
    assert r.status_code == 404
    assert r.json()["error"]["code"] == "PAYLINK_NOT_FOUND"
