"""Audit seam. Every privileged access (search / entity view) emits exactly one ``AuditRecord``.

The real system of record is ``audit-log-service`` (work13), which is not built yet — so this is a
drop-in seam (mirrors merchant-onboarding's event-publisher stub). ``LogAuditSink`` writes one
structured JSON line per access; when work13 lands, an ``HttpAuditSink`` POSTing the same record to
``audit-log-service /v1/audit-log`` is selected via ``ADMIN_AUDIT_SINK_MODE`` with no call-site
changes. Because :class:`app.domain.search_service.SearchService` and
:class:`app.domain.entity_service.EntityService` are the only call sites and each emits once, every
read is audited by construction.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass
from typing import Protocol

from app.config import Settings
from app.logging import get_logger

log = get_logger("admin.audit")


@dataclass(frozen=True)
class AuditRecord:
    actor: str  # JWT sub of the staff member
    actor_scopes: list[str]
    action: str  # e.g. admin.search | admin.view.user
    resource_type: str
    resource_id: str | None
    query: str | None
    result: str  # ok | not_found | degraded | error
    trace_id: str
    ip: str | None = None
    user_agent: str | None = None


class AuditSink(Protocol):
    async def emit(self, record: AuditRecord) -> None: ...


class LogAuditSink:
    """Structured-JSON audit sink — the work13 (audit-log-service) drop-in."""

    async def emit(self, record: AuditRecord) -> None:
        log.info("audit", **asdict(record))


class NoopAuditSink:
    """Inert sink (tests / explicit opt-out)."""

    async def emit(self, record: AuditRecord) -> None:
        return None


def build_audit_sink(settings: Settings) -> AuditSink:
    if settings.audit_sink_mode == "noop":
        return NoopAuditSink()
    return LogAuditSink()
