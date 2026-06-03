"""Notification preferences API (work10 Account → Notifications tab).

Same scope + auth seam as the inbox (``app.api.v1.inbox``): JWT-verified at the gateway, which
injects ``X-Creator-Addr`` — so a caller only ever reads/writes their own preferences. Rides the
existing ``/v1/notifications`` gateway route (no new route needed). ``GET`` returns the effective
set (defaults merged in); ``PUT`` is a patch — only the keys you send change — and is idempotent.
"""

from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1.schemas import UpdatePreferencesRequest
from app.deps import CallerDep, RepoDep
from app.domain.preferences import Preferences, merge_preferences

router = APIRouter()


def _view(prefs: Preferences, updated_at: datetime | None) -> dict[str, object]:
    return {
        "channels": prefs.channels,
        "events": prefs.events,
        "updated_at": updated_at.isoformat() if updated_at is not None else None,
    }


@router.get("/v1/notifications/preferences")
async def get_preferences(caller: CallerDep, repo: RepoDep) -> JSONResponse:
    row = await repo.get_preferences(caller)
    if row is None:
        # No row yet → all enabled (opt-out default); updated_at stays null.
        return JSONResponse(status_code=200, content=_view(merge_preferences(None, None), None))
    prefs = merge_preferences(row.channels, row.events)
    return JSONResponse(status_code=200, content=_view(prefs, row.updated_at))


@router.put("/v1/notifications/preferences")
async def update_preferences(
    caller: CallerDep, repo: RepoDep, body: UpdatePreferencesRequest
) -> JSONResponse:
    current = await repo.get_preferences(caller)
    base_channels = dict(current.channels) if current is not None else {}
    base_events = dict(current.events) if current is not None else {}
    # Patch the caller's changes over their current set, then normalize to the full effective set.
    merged = merge_preferences(
        {**base_channels, **(body.channels or {})},
        {**base_events, **(body.events or {})},
    )
    row = await repo.upsert_preferences(caller, channels=merged.channels, events=merged.events)
    prefs = merge_preferences(row.channels, row.events)
    return JSONResponse(status_code=200, content=_view(prefs, row.updated_at))
