"""Consumer-side dedupe helpers for the at-least-once event bus (work15).

The bus delivers each event at least once, so handlers MUST be idempotent. The consumer ``handle``
chokepoint receives only ``(name, payload)`` — not the envelope id — so the caller supplies a stable
dedupe key (a ``proof_hash``, a business id, or ``fingerprint(payload)``).

- :class:`RedisDedupe` — cheap best-effort short-circuit; skip repeated NON-DB work for a seen
  event. Best-effort: the marker has a TTL and Redis can evict it, so a very-late redelivery may
  slip through.
- :class:`DbDedupe` — durable exactly-once *effect*; the dedupe row is inserted on the caller's OWN
  transaction, so the mark and the handler's write commit together (survives Redis loss / restart).
"""

from __future__ import annotations

import re
from collections.abc import Awaitable, Callable
from typing import Any, Protocol, TypeVar

import sqlalchemy as sa

from .store import RedisLike

T = TypeVar("T")


class RedisDedupe:
    """Best-effort consumer dedupe backed by Redis SETNX (key ``idemc:<service>:<scope>:<key>``)."""

    def __init__(self, redis: RedisLike, service: str, ttl_seconds: int) -> None:
        self._redis = redis
        self._service = service
        self._ttl = ttl_seconds

    def _key(self, scope: str, dedupe_key: str) -> str:
        return f"idemc:{self._service}:{scope}:{dedupe_key}"

    async def seen_before(self, scope: str, dedupe_key: str) -> bool:
        """Report whether dedupe_key was already marked under scope, without claiming it."""
        return (await self._redis.get(self._key(scope, dedupe_key))) is not None

    async def run_once(
        self, scope: str, dedupe_key: str, action: Callable[[], Awaitable[T]]
    ) -> T | None:
        """Run action at most once per (scope, dedupe_key): the first caller wins the SETNX
        marker and runs action; a later caller with the same key is skipped (returns ``None``).
        If action raises, the marker is removed so the redelivered event retries (no poison-lock);
        the exception propagates so the bus does not commit the offset.
        """
        rkey = self._key(scope, dedupe_key)
        reserved = await self._redis.set(rkey, "1", nx=True, ex=self._ttl)
        if not reserved:
            return None
        try:
            return await action()
        except Exception:
            await self._redis.delete(rkey)
            raise


class Executor(Protocol):
    """The subset of a SQLAlchemy AsyncConnection/AsyncSession used by DbDedupe — so the dedupe row
    is written on the caller's own unit of work (and committed by the caller, never here)."""

    async def execute(self, statement: Any, parameters: Any = ...) -> Any: ...


_SAFE_IDENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$")


class DbDedupe:
    """Durable consumer dedupe backed by a Postgres table (default ``processed_events``; create it
    from the shipped migrations/processed_events.sql in the service's own schema). Gives a true
    exactly-once *effect*: the dedupe row commits atomically with the handler's write on the
    caller's transaction.
    """

    def __init__(self, table: str = "processed_events") -> None:
        # The table name is interpolated into SQL (identifiers can't be bound), so it must be a
        # trusted constant — a non-identifier value falls back to the default rather than reaching
        # the database.
        self._table = table if _SAFE_IDENT.match(table) else "processed_events"

    async def run_once(
        self, conn: Executor, scope: str, dedupe_key: str, action: Callable[[], Awaitable[T]]
    ) -> tuple[bool, T | None]:
        """Insert (scope, dedupe_key) on conn and run action only when the row is new. On a
        duplicate (the event was processed before) returns ``(False, None)`` without running action.
        The caller commits conn; the mark and action's writes commit together, so a redelivery never
        re-applies the effect. Returns ``(True, result)`` when action ran.
        """
        result = await conn.execute(
            sa.text(
                f"INSERT INTO {self._table} (scope, dedupe_key) VALUES (:scope, :key) "
                "ON CONFLICT (scope, dedupe_key) DO NOTHING"
            ),
            {"scope": scope, "key": dedupe_key},
        )
        if result.rowcount == 0:
            return False, None
        return True, await action()
