"""Async JSON-RPC 2.0 client for the lVM (``paylink-chain/internal/rpc``).

Transport/JSON-RPC errors surface as ``AppError(CHAIN_UNAVAILABLE)``; a "paylink not found" /
"receipt not found" RPC error maps to ``None`` so callers can treat absence normally.
"""

from __future__ import annotations

from typing import Any

import httpx

from app.chain.wire import go_json
from app.errors import AppError, ErrorCode


class RpcError(Exception):
    def __init__(self, code: int, message: str) -> None:
        self.code = code
        self.message = message
        super().__init__(f"rpc error {code}: {message}")


class ChainClient:
    def __init__(self, base_url: str, http: httpx.AsyncClient) -> None:
        self._base = base_url
        self._http = http
        self._id = 0

    async def _call(self, method: str, params: Any) -> Any:
        self._id += 1
        body = {"jsonrpc": "2.0", "method": method, "params": params, "id": self._id}
        try:
            resp = await self._http.post(
                self._base, content=go_json(body), headers={"Content-Type": "application/json"}
            )
        except httpx.HTTPError as exc:
            raise AppError(ErrorCode.CHAIN_UNAVAILABLE, f"chain rpc unreachable: {exc}") from exc
        if resp.status_code != 200:
            raise AppError(
                ErrorCode.CHAIN_UNAVAILABLE, f"chain rpc returned http {resp.status_code}"
            )
        data = resp.json()
        err = data.get("error")
        if err:
            raise RpcError(int(err.get("code", 0)), str(err.get("message", "rpc error")))
        return data.get("result")

    async def chain_height(self) -> int:
        return int(await self._call("paylink_chainHeight", {}))

    async def get_nonce(self, address: str) -> int:
        return int(await self._call("paylink_getNonce", {"address": address}))

    async def send_transaction(self, tx: dict[str, Any]) -> str:
        result = await self._call("paylink_sendTransaction", tx)
        return str(result["txHash"])

    async def get_paylink(self, pl_id: str) -> dict[str, Any] | None:
        try:
            return await self._call("paylink_getPayLink", {"id": pl_id})
        except RpcError as exc:
            if "not found" in exc.message.lower():
                return None
            raise AppError(ErrorCode.CHAIN_UNAVAILABLE, exc.message) from exc

    async def get_paylinks_by_creator(
        self, creator: str, limit: int = 100, offset: int = 0
    ) -> list[dict[str, Any]]:
        return await self._list("paylink_getPayLinksByCreator", "creator", creator, limit, offset)

    async def get_paylinks_by_receiver(
        self, receiver: str, limit: int = 100, offset: int = 0
    ) -> list[dict[str, Any]]:
        return await self._list(
            "paylink_getPayLinksByReceiver", "receiver", receiver, limit, offset
        )

    async def get_paylinks_by_status(
        self, status: str, limit: int = 100, offset: int = 0
    ) -> list[dict[str, Any]]:
        return await self._list("paylink_getPayLinksByStatus", "status", status, limit, offset)

    async def _list(
        self, method: str, field: str, value: str, limit: int, offset: int
    ) -> list[dict[str, Any]]:
        result = await self._call(method, {field: value, "limit": limit, "offset": offset})
        return list(result) if result else []

    async def get_receipt(self, tx_hash: str) -> dict[str, Any] | None:
        try:
            return await self._call("paylink_getTransactionReceipt", {"hash": tx_hash})
        except RpcError as exc:
            if "not found" in exc.message.lower():
                return None
            raise AppError(ErrorCode.CHAIN_UNAVAILABLE, exc.message) from exc
