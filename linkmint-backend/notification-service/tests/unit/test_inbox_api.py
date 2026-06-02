"""The public in-app notification center API (FE work07) over in-memory fakes.

Covers the address scoping (X-Creator-Addr), the intake→inbox write path, read/mark-read/mark-all,
ownership isolation, and the dedupe guarantee.
"""

from __future__ import annotations

import uuid
from typing import Any

from fastapi.testclient import TestClient

CREATOR = "0x00000000000000000000000000000000000000ab"
OTHER = "0x00000000000000000000000000000000000000cd"


def _intake(**over: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "event_kind": "paylink.created",
        "recipient_addr": CREATOR,
        "data": {"pl_id": "0xpl1", "amount": 1500, "currency": "KES", "dedupe_id": "0xpl1:created"},
    }
    body.update(over)
    return body


def _hdr(addr: str = CREATOR) -> dict[str, str]:
    return {"X-Creator-Addr": addr}


def test_list_requires_creator_addr(client: TestClient) -> None:
    resp = client.get("/v1/notifications")
    assert resp.status_code == 401
    assert resp.json()["error"]["code"] == "UNAUTHORIZED"


def test_intake_writes_inbox_then_list_returns_it(client: TestClient) -> None:
    posted = client.post("/v1/notifications", json=_intake())
    assert posted.status_code == 201
    assert len(posted.json()["delivery_ids"]) == 1  # the inbox notification id

    resp = client.get("/v1/notifications", headers=_hdr())
    assert resp.status_code == 200
    body = resp.json()
    assert body["next_cursor"] is None
    assert len(body["items"]) == 1
    item = body["items"][0]
    assert item["kind"] == "info"
    assert item["title"] == "PayLink created"
    assert "0xpl1" in item["body"]
    assert item["read"] is False
    assert set(item.keys()) == {"id", "kind", "title", "body", "href", "read", "created_at"}


def test_list_is_scoped_by_creator(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake())
    # A different caller sees none of CREATOR's notifications.
    resp = client.get("/v1/notifications", headers=_hdr(OTHER))
    assert resp.status_code == 200
    assert resp.json()["items"] == []


def test_creator_addr_is_case_insensitive(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake(recipient_addr=CREATOR.upper()))
    resp = client.get("/v1/notifications", headers=_hdr(CREATOR.upper()))
    assert len(resp.json()["items"]) == 1


def test_mark_read(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake())
    nid = client.get("/v1/notifications", headers=_hdr()).json()["items"][0]["id"]

    marked = client.post(f"/v1/notifications/{nid}/read", headers=_hdr())
    assert marked.status_code == 200
    assert marked.json()["read"] is True

    again = client.get("/v1/notifications", headers=_hdr()).json()["items"][0]
    assert again["read"] is True


def test_mark_read_unknown_404(client: TestClient) -> None:
    resp = client.post(f"/v1/notifications/{uuid.uuid4()}/read", headers=_hdr())
    assert resp.status_code == 404
    assert resp.json()["error"]["code"] == "NOTIFICATION_NOT_FOUND"


def test_mark_read_other_owner_404(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake())
    nid = client.get("/v1/notifications", headers=_hdr()).json()["items"][0]["id"]
    # OTHER cannot mark CREATOR's notification read — indistinguishable from "not found".
    resp = client.post(f"/v1/notifications/{nid}/read", headers=_hdr(OTHER))
    assert resp.status_code == 404


def test_mark_all_read(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake())
    client.post(
        "/v1/notifications",
        json=_intake(
            event_kind="paylink.verified",
            data={
                "pl_id": "0xpl1",
                "amount": 1500,
                "currency": "KES",
                "dedupe_id": "0xpl1:verified",
            },
        ),
    )
    resp = client.post("/v1/notifications/read-all", headers=_hdr())
    assert resp.status_code == 200
    assert resp.json()["count"] == 2
    items = client.get("/v1/notifications", headers=_hdr()).json()["items"]
    assert all(i["read"] for i in items)


def test_inbox_dedupe_no_double_post(client: TestClient) -> None:
    client.post("/v1/notifications", json=_intake())
    client.post("/v1/notifications", json=_intake())  # same event + recipient + dedupe_id
    items = client.get("/v1/notifications", headers=_hdr()).json()["items"]
    assert len(items) == 1


def test_explicit_title_body_override_derived_copy(client: TestClient) -> None:
    client.post(
        "/v1/notifications",
        json=_intake(title="Custom heading", body="Custom message", href="/dashboard/paylinks"),
    )
    item = client.get("/v1/notifications", headers=_hdr()).json()["items"][0]
    assert item["title"] == "Custom heading"
    assert item["body"] == "Custom message"
    assert item["href"] == "/dashboard/paylinks"


def test_intake_supports_both_inbox_and_delivery(client: TestClient, enqueue_spy: Any) -> None:
    # An event carrying BOTH recipient_addr (inbox) and user_id+contact (SMS/email) feeds both.
    resp = client.post(
        "/v1/notifications",
        json=_intake(
            event_kind="paylink.verified",
            user_id=str(uuid.uuid4()),
            contact={"phone": "+254712345678", "email": "jane@example.com"},
            data={"pl_id": "0xpl1", "amount": 1500, "currency": "KES", "dedupe_id": "0xpl1:v"},
        ),
    )
    assert resp.status_code == 201
    # 1 inbox + 2 deliveries (sms + email).
    assert len(resp.json()["delivery_ids"]) == 3
    assert len(enqueue_spy.ids) == 2  # only the SMS/email deliveries are enqueued
    inbox = client.get("/v1/notifications", headers=_hdr()).json()["items"]
    assert len(inbox) == 1
    assert inbox[0]["kind"] == "success"
