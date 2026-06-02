"""Redis-backed Idempotency-Key store for state-mutating endpoints (24h TTL).

A request that re-presents the same key+body gets the cached response (replay). The same key with a
*different* body is a conflict (IdempotencyConflict, reason ``body_mismatch`` → map to 409). A key
whose first request is still in flight is also a conflict (reason ``in_flight``). Keys are
namespaced per service+route so create/cancel never collide and the cache is safe on a Redis shared
with other services. The Go counterpart (idempotency-go) uses the identical key scheme and JSON
record shape.
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any, Protocol

from .errors import IdempotencyConflict


class RedisLike(Protocol):
    async def set(
        self, name: str, value: str, *, nx: bool = ..., ex: int | None = ...
    ) -> bool | None: ...
    async def get(self, name: str) -> str | None: ...
    async def delete(self, *names: str) -> int: ...
    async def ping(self) -> bool: ...


@dataclass(frozen=True)
class CachedResponse:
    http_status: int
    body: dict[str, Any]


def fingerprint(payload: Any) -> str:
    """Stable SHA-256 over a JSON-canonicalized request body."""
    raw = json.dumps(payload, sort_keys=True, separators=(",", ":"), default=str)
    return hashlib.sha256(raw.encode()).hexdigest()


class IdempotencyStore:
    def __init__(self, redis: RedisLike, service: str, ttl_seconds: int) -> None:
        self._redis = redis
        self._service = service
        self._ttl = ttl_seconds

    def _key(self, route: str, key: str) -> str:
        return f"idem:{self._service}:{route}:{key}"

    async def begin(self, route: str, key: str, fp: str) -> CachedResponse | None:
        """Reserve the key, or surface a prior result / conflict.

        Returns a :class:`CachedResponse` when a completed result should be replayed, or ``None``
        when the caller now owns the key and should perform the work then call :meth:`complete`.
        Raises :class:`IdempotencyConflict` on a body mismatch or an in-flight duplicate.
        """
        rkey = self._key(route, key)
        reserved = await self._redis.set(
            rkey, json.dumps({"state": "pending", "fp": fp}), nx=True, ex=self._ttl
        )
        if reserved:
            return None

        raw = await self._redis.get(rkey)
        if raw is None:
            # Reservation expired between SET NX and GET — treat as a fresh owner.
            await self._redis.set(rkey, json.dumps({"state": "pending", "fp": fp}), ex=self._ttl)
            return None

        data = json.loads(raw)
        if data.get("fp") != fp:
            raise IdempotencyConflict(
                "Idempotency-Key was already used with a different request body",
                reason="body_mismatch",
            )
        if data.get("state") == "completed":
            return CachedResponse(http_status=data["http_status"], body=data["body"])
        raise IdempotencyConflict(
            "a request with this Idempotency-Key is still in progress", reason="in_flight"
        )

    async def complete(
        self, route: str, key: str, fp: str, http_status: int, body: dict[str, Any]
    ) -> None:
        rkey = self._key(route, key)
        await self._redis.set(
            rkey,
            json.dumps({"state": "completed", "fp": fp, "http_status": http_status, "body": body}),
            ex=self._ttl,
        )

    async def release(self, route: str, key: str) -> None:
        """Drop a pending reservation so a failed request can be retried with the same key."""
        await self._redis.delete(self._key(route, key))
