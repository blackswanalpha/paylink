from __future__ import annotations

import httpx
import pytest
import respx

from app.providers.base import UpstreamError
from app.providers.http import (
    HttpMerchantProvider,
    HttpPaylinkProvider,
    HttpPaymentProvider,
    HttpUserProvider,
    UpstreamClient,
    build_registry,
)
from tests._support import make_settings


def _user_provider(client: httpx.AsyncClient) -> HttpUserProvider:
    return HttpUserProvider(UpstreamClient(client, "http://identity", "identity-service"))


@respx.mock
async def test_user_get_and_404() -> None:
    respx.get("http://identity/internal/admin/users/u1").mock(
        return_value=httpx.Response(
            200, json={"user_id": "u1", "email": "a@b.c", "status": "ACTIVE", "kyc_tier": 1}
        )
    )
    respx.get("http://identity/internal/admin/users/missing").mock(
        return_value=httpx.Response(404, json={"error": {}})
    )
    async with httpx.AsyncClient() as client:
        prov = _user_provider(client)
        view = await prov.get("u1")
        assert view is not None and view.data["email"] == "a@b.c"
        assert await prov.get("missing") is None


@respx.mock
async def test_user_search_maps_hits() -> None:
    respx.get("http://identity/internal/admin/users").mock(
        return_value=httpx.Response(
            200,
            json={
                "items": [{"user_id": "u1", "email": "a@b.c", "status": "ACTIVE", "kyc_tier": 2}]
            },
        )
    )
    async with httpx.AsyncClient() as client:
        hits = await _user_provider(client).search("a", 20)
    assert len(hits) == 1
    assert hits[0].type == "user" and hits[0].label == "a@b.c"
    assert hits[0].status == "ACTIVE" and hits[0].secondary["kyc_tier"] == "2"


@respx.mock
async def test_non_404_error_raises_upstream_error() -> None:
    respx.get("http://identity/internal/admin/users/x").mock(
        return_value=httpx.Response(500, text="boom")
    )
    async with httpx.AsyncClient() as client:
        with pytest.raises(UpstreamError):
            await _user_provider(client).get("x")


@respx.mock
async def test_network_failure_raises_upstream_error() -> None:
    respx.get("http://identity/internal/admin/users/x").mock(side_effect=httpx.ConnectError("down"))
    async with httpx.AsyncClient() as client:
        with pytest.raises(UpstreamError):
            await _user_provider(client).get("x")


@respx.mock
async def test_merchant_and_payment_search_map_hits() -> None:
    respx.get("http://m/internal/admin/merchants").mock(
        return_value=httpx.Response(
            200,
            json={
                "items": [
                    {
                        "merchant_id": "m1",
                        "business_name": "Acme",
                        "status": "ACTIVE",
                        "org_id": "o1",
                        "fee_tier": "standard",
                        "country": "KE",
                    }
                ]
            },
        )
    )
    respx.get("http://p/internal/admin/payments").mock(
        return_value=httpx.Response(
            200,
            json={
                "items": [{"id": "p1", "status": "SETTLED", "paylink_id": "0xabc", "rail": "mpesa"}]
            },
        )
    )
    async with httpx.AsyncClient() as client:
        mh = await HttpMerchantProvider(
            UpstreamClient(client, "http://m", "merchant-onboarding")
        ).search("acme", 20)
        ph = await HttpPaymentProvider(
            UpstreamClient(client, "http://p", "payment-orchestrator")
        ).search("settled", 20)
    assert mh[0].type == "merchant" and mh[0].secondary["org_id"] == "o1"
    assert ph[0].type == "payment" and ph[0].secondary["rail"] == "mpesa"


@respx.mock
async def test_paylink_search_by_id() -> None:
    plid = "0x" + "a" * 64
    respx.get(f"http://pl/v1/paylinks/{plid}").mock(
        return_value=httpx.Response(
            200, json={"pl_id": plid, "status": "CREATED", "amount": "100", "currency": "KES"}
        )
    )
    async with httpx.AsyncClient() as client:
        hits = await HttpPaylinkProvider(
            UpstreamClient(client, "http://pl", "paylink-service")
        ).search(plid, 20)
    assert len(hits) == 1 and hits[0].id == plid and hits[0].secondary["currency"] == "KES"


@respx.mock
async def test_paylink_search_by_filter() -> None:
    respx.get("http://pl/v1/paylinks").mock(
        return_value=httpx.Response(200, json={"items": [{"pl_id": "0xabc", "status": "CREATED"}]})
    )
    async with httpx.AsyncClient() as client:
        hits = await HttpPaylinkProvider(
            UpstreamClient(client, "http://pl", "paylink-service")
        ).search(
            "0xshort", 20
        )  # not 66 chars → filter path (creator first)
    assert hits and hits[0].type == "paylink"


@respx.mock
async def test_paylink_search_no_match_returns_empty() -> None:
    respx.get("http://pl/v1/paylinks").mock(return_value=httpx.Response(200, json={"items": []}))
    async with httpx.AsyncClient() as client:
        hits = await HttpPaylinkProvider(
            UpstreamClient(client, "http://pl", "paylink-service")
        ).search("nothing", 20)
    assert hits == []


async def test_build_registry_wires_four_providers() -> None:
    async with httpx.AsyncClient() as client:
        registry = build_registry(make_settings(), client)
    assert {p.entity_type for p in registry.all()} == {"user", "merchant", "paylink", "payment"}
