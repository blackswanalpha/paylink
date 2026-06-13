"""Upstream HTTP clients (payment-orchestrator + paylink-service) via respx."""

from __future__ import annotations

import httpx
import pytest
import respx

from app.errors import AppError, ErrorCode
from app.paylinks.client import HttpPaylinksClient
from app.payments.client import HttpPaymentsClient

PAY_BASE = "http://orchestrator:8080"
PL_BASE = "http://paylink:8000"


@respx.mock
async def test_payments_get_ok() -> None:
    respx.get(f"{PAY_BASE}/v1/payments/p1").mock(
        return_value=httpx.Response(
            200, json={"id": "p1", "paylink_id": "0xpl", "rail": "mpesa", "status": "SETTLED"}
        )
    )
    async with httpx.AsyncClient() as c:
        info = await HttpPaymentsClient(PAY_BASE, c).get("p1")
    assert info is not None and info.rail == "mpesa" and info.status == "SETTLED"


@respx.mock
async def test_payments_get_404_is_none() -> None:
    respx.get(f"{PAY_BASE}/v1/payments/missing").mock(return_value=httpx.Response(404))
    async with httpx.AsyncClient() as c:
        assert await HttpPaymentsClient(PAY_BASE, c).get("missing") is None


@respx.mock
async def test_payments_get_500_raises() -> None:
    respx.get(f"{PAY_BASE}/v1/payments/x").mock(return_value=httpx.Response(503))
    async with httpx.AsyncClient() as c:
        with pytest.raises(AppError) as exc:
            await HttpPaymentsClient(PAY_BASE, c).get("x")
    assert exc.value.code == ErrorCode.UPSTREAM_UNAVAILABLE


@respx.mock
async def test_payments_network_error_raises() -> None:
    respx.get(f"{PAY_BASE}/v1/payments/x").mock(side_effect=httpx.ConnectError("down"))
    async with httpx.AsyncClient() as c:
        with pytest.raises(AppError) as exc:
            await HttpPaymentsClient(PAY_BASE, c).get("x")
    assert exc.value.code == ErrorCode.UPSTREAM_UNAVAILABLE


@respx.mock
async def test_paylinks_get_amount_ok() -> None:
    respx.get(f"{PL_BASE}/v1/paylinks/0xpl").mock(
        return_value=httpx.Response(200, json={"amount": 1500, "currency": "KES"})
    )
    async with httpx.AsyncClient() as c:
        amt = await HttpPaylinksClient(PL_BASE, c).get_amount("0xpl")
    assert amt is not None and amt.amount_minor == 1500


@respx.mock
async def test_paylinks_404_is_none() -> None:
    respx.get(f"{PL_BASE}/v1/paylinks/x").mock(return_value=httpx.Response(404))
    async with httpx.AsyncClient() as c:
        assert await HttpPaylinksClient(PL_BASE, c).get_amount("x") is None


@respx.mock
async def test_paylinks_missing_amount_is_none() -> None:
    respx.get(f"{PL_BASE}/v1/paylinks/x").mock(return_value=httpx.Response(200, json={"foo": 1}))
    async with httpx.AsyncClient() as c:
        assert await HttpPaylinksClient(PL_BASE, c).get_amount("x") is None


@respx.mock
async def test_paylinks_network_error_is_none() -> None:
    respx.get(f"{PL_BASE}/v1/paylinks/x").mock(side_effect=httpx.ConnectError("down"))
    async with httpx.AsyncClient() as c:
        assert await HttpPaylinksClient(PL_BASE, c).get_amount("x") is None
