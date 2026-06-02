"""The intake + delivery-view API (TestClient over in-memory fakes)."""

from __future__ import annotations

import uuid
from typing import Any

from fastapi.testclient import TestClient

from app.main import create_app
from tests._support import EnqueueSpy, FakeRepository, install_overrides, make_settings

CONTACT = {"phone": "+254712345678", "email": "jane@example.com"}
DATA = {"amount": "1500", "currency": "KES", "paylink_id": "pl_1", "dedupe_id": "pl_1"}


def _body(**over: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "event_kind": "paylink.verified",
        "user_id": str(uuid.uuid4()),
        "data": DATA,
        "contact": CONTACT,
    }
    body.update(over)
    return body


def test_healthz(client: TestClient) -> None:
    resp = client.get("/internal/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


def test_intake_creates_deliveries(client: TestClient, enqueue_spy: EnqueueSpy) -> None:
    resp = client.post("/v1/notifications", json=_body())
    assert resp.status_code == 201
    assert len(resp.json()["delivery_ids"]) == 2
    assert len(enqueue_spy.ids) == 2


def test_intake_idempotent_replay(client: TestClient, enqueue_spy: EnqueueSpy) -> None:
    body = _body()
    headers = {"Idempotency-Key": "key-1"}
    r1 = client.post("/v1/notifications", json=body, headers=headers)
    r2 = client.post("/v1/notifications", json=body, headers=headers)
    assert r1.status_code == 201
    assert r1.json() == r2.json()  # cached replay
    assert len(enqueue_spy.ids) == 2  # not 4 — the second call was served from cache


def test_intake_idempotency_conflict(client: TestClient) -> None:
    headers = {"Idempotency-Key": "key-2"}
    client.post("/v1/notifications", json=_body(), headers=headers)
    resp = client.post("/v1/notifications", json=_body(data={"x": "y"}), headers=headers)
    assert resp.status_code == 409
    assert resp.json()["error"]["code"] == "IDEMPOTENT_CONFLICT"


def test_unknown_event_returns_empty(client: TestClient) -> None:
    resp = client.post("/v1/notifications", json=_body(event_kind="mystery.thing"))
    assert resp.status_code == 201
    assert resp.json()["delivery_ids"] == []


def test_get_delivery_masks_recipient(client: TestClient) -> None:
    ids = client.post("/v1/notifications", json=_body()).json()["delivery_ids"]
    resp = client.get(f"/internal/deliveries/{ids[0]}")
    assert resp.status_code == 200
    view = resp.json()
    assert "*" in view["recipient"]  # never the raw contact
    assert view["status"] == "QUEUED"


def test_get_delivery_unknown_404(client: TestClient) -> None:
    resp = client.get(f"/internal/deliveries/{uuid.uuid4()}")
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "DELIVERY_NOT_FOUND"


def test_get_delivery_bad_uuid_400(client: TestClient) -> None:
    resp = client.get("/internal/deliveries/not-a-uuid")
    assert resp.status_code == 400


def test_gate_requires_internal_token(
    fake_repo: FakeRepository, idem_store: Any, enqueue_spy: EnqueueSpy
) -> None:
    app = create_app(make_settings(internal_shared_secret="sek"))
    with TestClient(app) as c:
        install_overrides(app, fake_repo, idem_store, enqueue_spy)
        assert c.post("/v1/notifications", json=_body()).status_code == 401
        ok = c.post("/v1/notifications", json=_body(), headers={"X-Internal-Token": "sek"})
        assert ok.status_code == 201
    app.dependency_overrides.clear()
