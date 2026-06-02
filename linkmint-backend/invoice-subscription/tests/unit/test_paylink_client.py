"""PaylinkClient — the outbound paylink-service adapter (respx-mocked)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import httpx
import pytest
import respx

from app.paylinks.client import PaylinkClient, PaylinkError

_ADDR = "0x" + "a" * 40
_EXPIRY = datetime.now(UTC) + timedelta(days=1)


async def test_create_ok_and_forwards_creator_addr() -> None:
    async with httpx.AsyncClient() as http, respx.mock:
        route = respx.post("http://pl.test/v1/paylinks").mock(
            return_value=httpx.Response(201, json={"pl_id": "PLK_1", "status": "PENDING"})
        )
        client = PaylinkClient("http://pl.test", http, internal_token="t0ken")
        out = await client.create(
            receiver=_ADDR,
            amount=1000,
            currency="PLN",
            expiry=_EXPIRY,
            idempotency_key="invoice-abc",
        )
        assert out["pl_id"] == "PLK_1"
        req = route.calls[0].request
        assert req.headers["X-Creator-Addr"] == _ADDR
        assert req.headers["X-Internal-Token"] == "t0ken"
        assert req.headers["Idempotency-Key"] == "invoice-abc"


async def test_create_non_201_raises() -> None:
    async with httpx.AsyncClient() as http, respx.mock:
        respx.post("http://pl.test/v1/paylinks").mock(
            return_value=httpx.Response(400, json={"error": {}})
        )
        client = PaylinkClient("http://pl.test", http)
        with pytest.raises(PaylinkError):
            await client.create(receiver=_ADDR, amount=1, currency="PLN", expiry=_EXPIRY)


async def test_create_transport_error_raises() -> None:
    async with httpx.AsyncClient() as http, respx.mock:
        respx.post("http://pl.test/v1/paylinks").mock(side_effect=httpx.ConnectError("down"))
        client = PaylinkClient("http://pl.test", http)
        with pytest.raises(PaylinkError):
            await client.create(receiver=_ADDR, amount=1, currency="PLN", expiry=_EXPIRY)


async def test_create_bad_json_raises() -> None:
    async with httpx.AsyncClient() as http, respx.mock:
        respx.post("http://pl.test/v1/paylinks").mock(
            return_value=httpx.Response(201, content=b"not json")
        )
        client = PaylinkClient("http://pl.test", http)
        with pytest.raises(PaylinkError):
            await client.create(receiver=_ADDR, amount=1, currency="PLN", expiry=_EXPIRY)
