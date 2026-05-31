"""End-to-end against real Postgres + Redis (testcontainers); chain client is faked.

Exercises the real app: lifespan wiring, readiness probe, repository + cursor pagination over real
Postgres, idempotency over real Redis, and chain status read-through.
"""

from __future__ import annotations

from collections.abc import Iterator
from typing import Any

import pytest
from fastapi.testclient import TestClient

from app.chain.nonce import NonceManager
from app.config import Settings
from app.main import create_app
from tests._support import FakeChainClient

pytestmark = pytest.mark.integration

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


@pytest.fixture
def app_client(pg_url: str, redis_url: str, truncate: Any) -> Iterator[tuple[Any, FakeChainClient]]:
    settings = Settings(
        database_url=pg_url,
        redis_url=redis_url,
        chain_rpc_url="http://localhost:8545/",
        chain_submit_enabled=True,
        signer_mode="service_key",
        chain_signer_key=None,
        event_publisher_mode="noop",
    )
    app = create_app(settings)
    with TestClient(app) as test_client:
        fake = FakeChainClient()
        test_client.app.state.chain_client = fake
        test_client.app.state.nonces = NonceManager(fake)  # type: ignore[arg-type]
        yield test_client, fake


def test_readyz_reports_backends_ok(app_client: tuple[Any, FakeChainClient]) -> None:
    client, _ = app_client
    r = client.get("/internal/readyz")
    assert r.status_code == 200
    checks = r.json()["checks"]
    assert checks["db"] == "ok"
    assert checks["redis"] == "ok"


def test_create_get_readthrough_and_idempotency(app_client: tuple[Any, FakeChainClient]) -> None:
    client, fake = app_client

    r = client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "k1"})
    assert r.status_code == 201
    pl_id = r.json()["pl_id"]
    assert r.json()["status"] == "PENDING"
    assert r.json()["chain_tx_hash"]
    assert len(fake.sent) == 1

    assert client.get(f"/v1/paylinks/{pl_id}").json()["status"] == "PENDING"

    # chain read-through reflects settlement
    fake.paylinks[pl_id] = {
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

    # idempotent replay vs conflict
    assert (
        client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "k1"}).json()["pl_id"]
        == pl_id
    )
    conflict = client.post(
        "/v1/paylinks", json=_body(amount=999), headers={"Idempotency-Key": "k1"}
    )
    assert conflict.status_code == 409


def test_list_cursor_pagination_real_db(app_client: tuple[Any, FakeChainClient]) -> None:
    client, _ = app_client
    for i in range(3):
        client.post(
            "/v1/paylinks", json=_body(amount=100 + i), headers={"Idempotency-Key": f"p{i}"}
        )
    page1 = client.get("/v1/paylinks", params={"limit": 2}).json()
    assert len(page1["items"]) == 2
    assert page1["next_cursor"]
    page2 = client.get("/v1/paylinks", params={"limit": 2, "cursor": page1["next_cursor"]}).json()
    assert len(page2["items"]) == 1


def test_cancel_real_db(app_client: tuple[Any, FakeChainClient]) -> None:
    client, _ = app_client
    pl_id = client.post("/v1/paylinks", json=_body(), headers={"Idempotency-Key": "cc"}).json()[
        "pl_id"
    ]
    r = client.post(f"/v1/paylinks/{pl_id}/cancel", headers={"Idempotency-Key": "cancel-cc"})
    assert r.status_code == 200
    assert r.json()["status"] == "CANCELLED"
    assert client.get(f"/v1/paylinks/{pl_id}").json()["status"] == "CANCELLED"


def test_error_envelope_real(app_client: tuple[Any, FakeChainClient]) -> None:
    client, _ = app_client
    r = client.post("/v1/paylinks", json=_body(amount=-5))
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "INVALID_PAYLOAD"
