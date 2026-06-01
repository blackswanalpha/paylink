"""Shared idempotency wrapper for state-mutating endpoints (mirrors merchant-onboarding)."""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from typing import Any

from fastapi.responses import JSONResponse

from app.errors import AppError, ErrorCode
from app.idempotency import IdempotencyStore, fingerprint


def parse_uuid(value: str, *, field: str = "id") -> uuid.UUID:
    try:
        return uuid.UUID(value)
    except ValueError as exc:
        raise AppError(
            ErrorCode.INVALID_PAYLOAD, f"invalid {field}", details={"field": field}
        ) from exc


async def idempotent(
    idem: IdempotencyStore,
    route: str,
    key: str | None,
    payload: dict[str, Any],
    status: int,
    work: Callable[[], Awaitable[dict[str, Any]]],
) -> JSONResponse:
    """Run ``work`` once per Idempotency-Key, replaying the cached response on retry.

    ``work`` returns the response body dict. A failure releases the reservation so the key can be
    retried; a body mismatch / in-flight duplicate surfaces as a 409 from the store.
    """
    fp = fingerprint(payload)
    if key:
        cached = await idem.begin(route, key, fp)
        if cached is not None:
            return JSONResponse(status_code=cached.http_status, content=cached.body)
    try:
        body = await work()
    except Exception:
        if key:
            await idem.release(route, key)
        raise
    if key:
        await idem.complete(route, key, fp, status, body)
    return JSONResponse(status_code=status, content=body)
