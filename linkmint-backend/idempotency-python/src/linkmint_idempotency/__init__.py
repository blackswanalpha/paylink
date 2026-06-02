"""linkmint_idempotency — LinkMint's shared idempotency helper (work17).

A Redis-backed Idempotency-Key store for replay-safe state-mutating endpoints (24h TTL) plus
consumer-side dedupe helpers for the at-least-once event bus (work15): RedisDedupe (cheap
best-effort short-circuit) and DbDedupe (durable exactly-once effect on the caller's transaction).
Byte-compatible with idempotency-go — identical ``idem:<service>:<route>:<key>`` scheme and JSON
record shape — so Go and Python services share one Redis. The application-layer complement to the
chain's on-chain anti-replay (A.7), which stays the source of truth for settlement.
"""

from __future__ import annotations

from .consumer import DbDedupe, Executor, RedisDedupe
from .errors import IdempotencyConflict, IdempotencyError
from .store import CachedResponse, IdempotencyStore, RedisLike, fingerprint

__all__ = [
    "IdempotencyStore",
    "CachedResponse",
    "RedisLike",
    "fingerprint",
    "RedisDedupe",
    "DbDedupe",
    "Executor",
    "IdempotencyError",
    "IdempotencyConflict",
]
