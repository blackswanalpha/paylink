from __future__ import annotations

import uuid

import httpx
import pytest
import respx

from app.providers.base import UpstreamError
from app.providers.http import HttpKycProvider

BASE = "https://vendor.example"


def _provider(client: httpx.AsyncClient) -> HttpKycProvider:
    return HttpKycProvider("http", client, BASE)


@respx.mock
async def test_start_success() -> None:
    respx.post(f"{BASE}/sessions").mock(
        return_value=httpx.Response(
            200, json={"session_id": "sess-9", "provider_url": "https://vendor.example/v/sess-9"}
        )
    )
    async with httpx.AsyncClient() as client:
        started = await _provider(client).start(uuid.uuid4(), 2)
    assert started.session_id == "sess-9"
    assert started.provider_url == "https://vendor.example/v/sess-9"


@respx.mock
async def test_start_error_status_raises_upstream() -> None:
    respx.post(f"{BASE}/sessions").mock(return_value=httpx.Response(500))
    async with httpx.AsyncClient() as client:
        with pytest.raises(UpstreamError):
            await _provider(client).start(uuid.uuid4(), 1)


@respx.mock
async def test_start_network_error_raises_upstream() -> None:
    respx.post(f"{BASE}/sessions").mock(side_effect=httpx.ConnectError("boom"))
    async with httpx.AsyncClient() as client:
        with pytest.raises(UpstreamError):
            await _provider(client).start(uuid.uuid4(), 1)


async def test_parse_callback_passed_variants() -> None:
    async with httpx.AsyncClient() as client:
        p = _provider(client)
        uid = uuid.uuid4()
        for status in ("passed", "approved", "verified"):
            result = p.parse_callback(
                {"user_id": str(uid), "status": status, "tier": 2, "session_id": "s"}
            )
            assert result.passed is True
            assert result.tier_granted == 2
            assert result.provider_ref == "s"


async def test_parse_callback_failed() -> None:
    async with httpx.AsyncClient() as client:
        result = _provider(client).parse_callback(
            {"user_id": str(uuid.uuid4()), "status": "rejected"}
        )
    assert result.passed is False
    assert result.tier_granted == 1  # default
