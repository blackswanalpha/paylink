"""Per-address nonce manager.

Serializes nonce assignment for the service signer so concurrent submissions don't collide. The
chain assigns ``Nonce == state.GetNonce(from)``; we take ``max(chain_nonce, local_next)`` and only
advance the local counter when the submission inside the ``reserve`` block succeeds (so a failed
submit leaves no gap and is safely retried).
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from app.chain.client import ChainClient


class NonceManager:
    def __init__(self, chain: ChainClient) -> None:
        self._chain = chain
        self._lock = asyncio.Lock()
        self._next: dict[str, int] = {}

    @asynccontextmanager
    async def reserve(self, address: str) -> AsyncIterator[int]:
        async with self._lock:
            chain_nonce = await self._chain.get_nonce(address)
            nonce = max(chain_nonce, self._next.get(address, 0))
            yield nonce
            # Reached only if the submission in the with-body did not raise.
            self._next[address] = nonce + 1
