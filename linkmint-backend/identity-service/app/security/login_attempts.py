"""Redis-backed consecutive-failure counter (drives ``identity.auth.failed`` after N failures)."""

from __future__ import annotations

from typing import Protocol


class _RedisLike(Protocol):
    async def incr(self, name: str) -> int: ...
    async def expire(self, name: str, seconds: int) -> bool: ...
    async def delete(self, *names: str) -> int: ...


class FailedLoginCounter:
    def __init__(self, redis: _RedisLike, window_seconds: int = 15 * 60) -> None:
        self._redis = redis
        self._window = window_seconds

    @staticmethod
    def _key(identifier: str) -> str:
        return f"authfail:{identifier.lower()}"

    async def record(self, identifier: str) -> int:
        """Increment the failure count for an identifier; returns the new count."""
        key = self._key(identifier)
        count = await self._redis.incr(key)
        if count == 1:
            await self._redis.expire(key, self._window)
        return count

    async def reset(self, identifier: str) -> None:
        await self._redis.delete(self._key(identifier))
