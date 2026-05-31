"""`/v1/sessions` — list active sessions + per-session revoke."""

from __future__ import annotations

import uuid
from typing import Any

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/sessions", tags=["sessions"])


@router.get("", response_model=schemas.SessionListResponse)
async def list_sessions(
    services: ServicesDep, principal: PrincipalDep
) -> schemas.SessionListResponse:
    sessions = await services.sessions.list_active(uuid.UUID(principal.user_id))
    return schemas.SessionListResponse(
        items=[
            schemas.SessionResponse(
                session_id=str(s.session_id),
                user_agent=s.user_agent,
                ip=str(s.ip) if s.ip is not None else None,
                created_at=s.created_at,
                expires_at=s.expires_at,
                current=str(s.session_id) == principal.sid,
            )
            for s in sessions
        ]
    )


@router.delete("/{session_id}")
async def revoke_session(
    session_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    sid = parse_uuid(session_id, field="session_id")

    async def work() -> dict[str, Any]:
        await services.sessions.revoke(uuid.UUID(principal.user_id), sid)
        return {"status": "revoked", "session_id": session_id}

    return await idempotent(
        idem, "revoke_session", idempotency_key, {"session_id": session_id}, 200, work
    )
