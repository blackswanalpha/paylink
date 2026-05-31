from __future__ import annotations

from collections.abc import Callable

import httpx
import pytest

from app.chain.client import ChainClient
from app.errors import AppError, ErrorCode


def _client(
    handler: Callable[[httpx.Request], httpx.Response],
) -> tuple[ChainClient, httpx.AsyncClient]:
    http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ChainClient("http://chain/", http), http


def _ok(result: object) -> Callable[[httpx.Request], httpx.Response]:
    def handler(_req: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"jsonrpc": "2.0", "result": result, "id": 1})

    return handler


def _err(code: int, message: str) -> Callable[[httpx.Request], httpx.Response]:
    def handler(_req: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200, json={"jsonrpc": "2.0", "error": {"code": code, "message": message}, "id": 1}
        )

    return handler


async def test_get_nonce() -> None:
    client, http = _client(_ok(7))
    assert await client.get_nonce("0xabc") == 7
    await http.aclose()


async def test_send_transaction_returns_hash() -> None:
    client, http = _client(_ok({"txHash": "0xhash"}))
    assert await client.send_transaction({"type": 1}) == "0xhash"
    await http.aclose()


async def test_get_paylink_not_found_is_none() -> None:
    client, http = _client(_err(-32602, "paylink not found"))
    assert await client.get_paylink("0x1") is None
    await http.aclose()


async def test_other_rpc_error_is_chain_unavailable() -> None:
    client, http = _client(_err(-32603, "boom"))
    with pytest.raises(AppError) as exc:
        await client.get_paylink("0x1")
    assert exc.value.code is ErrorCode.CHAIN_UNAVAILABLE
    await http.aclose()


async def test_transport_error_is_chain_unavailable() -> None:
    def handler(_req: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused")

    client, http = _client(handler)
    with pytest.raises(AppError) as exc:
        await client.chain_height()
    assert exc.value.code is ErrorCode.CHAIN_UNAVAILABLE
    await http.aclose()


async def test_http_500_is_chain_unavailable() -> None:
    client, http = _client(lambda _req: httpx.Response(500, text="err"))
    with pytest.raises(AppError):
        await client.chain_height()
    await http.aclose()


async def test_list_empty_result_is_empty_list() -> None:
    client, http = _client(_ok(None))
    assert await client.get_paylinks_by_creator("0xabc") == []
    await http.aclose()


async def test_list_by_status_passes_through() -> None:
    client, http = _client(_ok([{"id": "0x1"}]))
    assert await client.get_paylinks_by_status("CREATED") == [{"id": "0x1"}]
    await http.aclose()
