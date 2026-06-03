"""Notification preferences API + its enforcement in the fan-out (work10).

Covers the address scoping (X-Creator-Addr), the defaults/patch/roundtrip of GET+PUT, and — the part
that makes preferences real — that a disabled event or channel actually suppresses the inbox write
and the SMS/email deliveries.
"""

from __future__ import annotations

import uuid
from typing import Any

from fastapi.testclient import TestClient

CREATOR = "0x00000000000000000000000000000000000000ab"
OTHER = "0x00000000000000000000000000000000000000cd"

ALL_CHANNELS = {"in_app", "email", "sms"}
ALL_EVENTS = {"paylink.created", "paylink.verified", "paylink.cancelled", "payment.failed"}


def _hdr(addr: str = CREATOR) -> dict[str, str]:
    return {"X-Creator-Addr": addr}


def _intake(**over: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "event_kind": "paylink.created",
        "recipient_addr": CREATOR,
        "data": {"pl_id": "0xpl1", "amount": 1500, "currency": "KES", "dedupe_id": "0xpl1:created"},
    }
    body.update(over)
    return body


# ── API: auth + defaults + roundtrip ────────────────────────────────────────────────────────────


def test_get_requires_creator_addr(client: TestClient) -> None:
    resp = client.get("/v1/notifications/preferences")
    assert resp.status_code == 401
    assert resp.json()["error"]["code"] == "UNAUTHORIZED"


def test_put_requires_creator_addr(client: TestClient) -> None:
    resp = client.put("/v1/notifications/preferences", json={"channels": {"email": False}})
    assert resp.status_code == 401


def test_get_defaults_when_unset(client: TestClient) -> None:
    resp = client.get("/v1/notifications/preferences", headers=_hdr())
    assert resp.status_code == 200
    body = resp.json()
    assert set(body["channels"]) == ALL_CHANNELS
    assert set(body["events"]) == ALL_EVENTS
    assert all(body["channels"].values()) and all(body["events"].values())
    assert body["updated_at"] is None


def test_put_then_get_roundtrips(client: TestClient) -> None:
    put = client.put(
        "/v1/notifications/preferences",
        headers=_hdr(),
        json={"channels": {"email": False}, "events": {"paylink.created": False}},
    )
    assert put.status_code == 200
    body = put.json()
    assert body["channels"] == {"in_app": True, "email": False, "sms": True}
    assert body["events"]["paylink.created"] is False
    assert body["events"]["paylink.verified"] is True
    assert body["updated_at"] is not None

    got = client.get("/v1/notifications/preferences", headers=_hdr()).json()
    assert got["channels"]["email"] is False
    assert got["events"]["paylink.created"] is False


def test_put_is_a_patch(client: TestClient) -> None:
    client.put("/v1/notifications/preferences", headers=_hdr(), json={"channels": {"email": False}})
    # A second patch on a different key must not resurrect the first.
    body = client.put(
        "/v1/notifications/preferences", headers=_hdr(), json={"channels": {"sms": False}}
    ).json()
    assert body["channels"] == {"in_app": True, "email": False, "sms": False}


def test_put_ignores_unknown_keys(client: TestClient) -> None:
    body = client.put(
        "/v1/notifications/preferences",
        headers=_hdr(),
        json={"channels": {"telepathy": False}, "events": {"world.peace": False}},
    ).json()
    assert set(body["channels"]) == ALL_CHANNELS
    assert "telepathy" not in body["channels"]
    assert all(body["channels"].values()) and all(body["events"].values())


def test_preferences_scoped_by_creator(client: TestClient) -> None:
    client.put("/v1/notifications/preferences", headers=_hdr(), json={"channels": {"email": False}})
    # OTHER has set nothing → still all-enabled defaults.
    other = client.get("/v1/notifications/preferences", headers=_hdr(OTHER)).json()
    assert other["channels"]["email"] is True
    assert other["updated_at"] is None


# ── Enforcement: a disabled event/channel suppresses delivery ────────────────────────────────────


def test_disabled_event_suppresses_inbox(client: TestClient) -> None:
    client.put(
        "/v1/notifications/preferences", headers=_hdr(), json={"events": {"paylink.created": False}}
    )
    posted = client.post("/v1/notifications", json=_intake())
    assert posted.status_code == 201
    assert posted.json()["delivery_ids"] == []  # nothing created
    assert client.get("/v1/notifications", headers=_hdr()).json()["items"] == []


def test_disabled_in_app_channel_suppresses_inbox(client: TestClient) -> None:
    client.put(
        "/v1/notifications/preferences", headers=_hdr(), json={"channels": {"in_app": False}}
    )
    client.post("/v1/notifications", json=_intake())
    assert client.get("/v1/notifications", headers=_hdr()).json()["items"] == []


def test_enabled_event_still_delivered(client: TestClient) -> None:
    # A sibling event the recipient did NOT disable still lands.
    client.put(
        "/v1/notifications/preferences", headers=_hdr(), json={"events": {"paylink.created": False}}
    )
    client.post(
        "/v1/notifications",
        json=_intake(
            event_kind="paylink.verified",
            data={"pl_id": "0xpl1", "amount": 1500, "currency": "KES", "dedupe_id": "0xpl1:v"},
        ),
    )
    items = client.get("/v1/notifications", headers=_hdr()).json()["items"]
    assert len(items) == 1
    assert items[0]["title"] == "PayLink settled"


def test_disabled_email_channel_suppresses_only_email(
    client: TestClient, enqueue_spy: Any
) -> None:
    client.put("/v1/notifications/preferences", headers=_hdr(), json={"channels": {"email": False}})
    # paylink.verified carries BOTH the inbox (recipient_addr) and SMS/email (user_id+contact).
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
    # inbox (in_app on) + sms (on) — email is suppressed.
    assert len(resp.json()["delivery_ids"]) == 2
    assert len(enqueue_spy.ids) == 1  # only the SMS delivery is enqueued
    assert len(client.get("/v1/notifications", headers=_hdr()).json()["items"]) == 1
