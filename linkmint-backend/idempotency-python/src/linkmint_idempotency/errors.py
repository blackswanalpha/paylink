"""Typed idempotency errors (mirror the idempotency-go sentinels). No web-framework import —
adopting services catch IdempotencyConflict and render their own envelope (409 IDEMPOTENT_CONFLICT).
"""

from __future__ import annotations


class IdempotencyError(Exception):
    """Base class for all idempotency errors."""


class IdempotencyConflict(IdempotencyError):
    """The same Idempotency-Key was re-presented with a different request body, or while a first
    request with that key is still in flight. Adopting services map this to 409 IDEMPOTENT_CONFLICT.

    ``reason`` is ``"body_mismatch"`` or ``"in_flight"``.
    """

    def __init__(self, message: str, *, reason: str) -> None:
        super().__init__(message)
        self.reason = reason
