"""Public in-app notification center API (FE work07).

User-facing + JWT-authenticated at the edge: the gateway verifies the bearer token and injects the
caller as ``X-Creator-Addr`` (the same seam paylink-service trusts), so every read here is scoped to
that address — a caller only ever sees / mutates their own notifications. Mutations are naturally
idempotent (mark-read), so no ``Idempotency-Key`` is required.
"""

from __future__ import annotations

import uuid

from fastapi import APIRouter, Query
from fastapi.responses import JSONResponse

from app.api.v1.schemas import InboxNotificationView
from app.db.models import InboxNotificationRow
from app.deps import CallerDep, RepoDep
from app.errors import AppError, ErrorCode

router = APIRouter()


def _view(row: InboxNotificationRow) -> dict[str, object]:
    # created_at is NOT NULL (server_default now()) so a persisted row always has it — the wire
    # contract (InboxNotificationView / SDK Notification) declares it non-nullable to match.
    return {
        "id": str(row.notification_id),
        "kind": row.kind,
        "title": row.title,
        "body": row.body,
        "href": row.href,
        "read": row.read,
        "created_at": row.created_at.isoformat(),
    }


@router.get("/v1/notifications")
async def list_notifications(
    caller: CallerDep,
    repo: RepoDep,
    limit: int = Query(default=20, ge=1, le=100),
    cursor: str | None = Query(default=None),
) -> JSONResponse:
    rows, next_cursor = await repo.list_inbox(caller, limit=limit, cursor=cursor)
    return JSONResponse(
        status_code=200,
        content={"items": [_view(row) for row in rows], "next_cursor": next_cursor},
    )


@router.post("/v1/notifications/{notification_id}/read")
async def mark_read(
    notification_id: uuid.UUID,
    caller: CallerDep,
    repo: RepoDep,
) -> JSONResponse:
    row = await repo.mark_inbox_read(caller, notification_id)
    if row is None:
        raise AppError(
            ErrorCode.NOTIFICATION_NOT_FOUND,
            "notification not found",
            details={"notification_id": str(notification_id)},
        )
    return JSONResponse(status_code=200, content=_view(row))


@router.post("/v1/notifications/read-all")
async def mark_all_read(caller: CallerDep, repo: RepoDep) -> JSONResponse:
    count = await repo.mark_all_inbox_read(caller)
    return JSONResponse(status_code=200, content={"count": count})


# Re-export for OpenAPI typing parity (the handlers return JSONResponse for explicit status codes).
__all__ = ["router", "InboxNotificationView"]
