"""Unit tests for the notification client (httpx MockTransport — no network, no Docker)."""

from __future__ import annotations

import json
from collections.abc import Callable

import httpx

from app.notifications.client import NotificationClient

Handler = Callable[[httpx.Request], httpx.Response]


def _client(handler: Handler, *, token: str | None = "tok") -> NotificationClient:
    http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return NotificationClient("http://notify:8095/", http, internal_token=token, timeout=2.0)


async def test_notify_posts_payload_token_and_dedupe() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/v1/notifications"
        assert request.headers["X-Internal-Token"] == "tok"
        body = json.loads(request.content)
        assert body["event_kind"] == "paylink.created"
        assert body["recipient_addr"] == "0xabc"
        assert body["data"] == {
            "pl_id": "0xpl",
            "amount": 1500,
            "dedupe_id": "0xpl:paylink.created",
        }
        assert body["href"] == "/dashboard/paylinks"
        return httpx.Response(201, json={"delivery_ids": ["n1"]})

    await _client(handler).notify(
        event_kind="paylink.created",
        recipient_addr="0xabc",
        data={"pl_id": "0xpl", "amount": 1500},
        dedupe_id="0xpl:paylink.created",
        href="/dashboard/paylinks",
    )


async def test_notify_omits_token_and_optional_fields() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert "x-internal-token" not in {k.lower() for k in request.headers}
        assert "title" not in body and "body" not in body and "href" not in body
        return httpx.Response(201, json={"delivery_ids": []})

    await _client(handler, token=None).notify(
        event_kind="paylink.verified",
        recipient_addr="0xabc",
        data={"pl_id": "0xpl"},
        dedupe_id="0xpl:paylink.verified",
    )


async def test_notify_swallows_non_2xx() -> None:
    # Best-effort: a non-2xx response must not raise.
    await _client(lambda _r: httpx.Response(503, text="down")).notify(
        event_kind="paylink.created",
        recipient_addr="0xabc",
        data={"pl_id": "0xpl"},
        dedupe_id="d",
    )


async def test_notify_swallows_transport_error() -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("boom")

    await _client(handler).notify(
        event_kind="paylink.created",
        recipient_addr="0xabc",
        data={"pl_id": "0xpl"},
        dedupe_id="d",
    )
