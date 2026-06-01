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

import httpx

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


class HttpAuditSink:
    """POST each ``AuditRecord`` to audit-log-service (work13) ``/v1/audit-log`` — the real system
    of record. Best-effort and bounded: a failed or slow emit logs a warning and returns; it never
    raises, so an audit-log outage can never break a privileged read. The record is mapped to the
    backendfeatures.md §2.17 intake shape — ``actor`` as the bare-string JWT sub (work13 maps it to
    ``{id, kind:"user"}``), ``resource`` as ``<type>:<id>``, and the residual fields in ``context``.
    """

    def __init__(
        self, client: httpx.AsyncClient, url: str, token: str | None, timeout: float
    ) -> None:
        self._client = client
        self._url = url.rstrip("/") + "/v1/audit-log"
        self._token = token
        self._timeout = timeout

    async def emit(self, record: AuditRecord) -> None:
        resource = record.resource_type
        if record.resource_id:
            resource = f"{record.resource_type}:{record.resource_id}"
        body = {
            "actor": record.actor,  # bare-string sub; work13 maps to {id, kind:"user"}
            "action": record.action,
            "resource": resource,
            "context": {
                "trace_id": record.trace_id,
                "ip": record.ip,
                "user_agent": record.user_agent,
                "query": record.query,
                "actor_scopes": record.actor_scopes,
                "result": record.result,
            },
        }
        headers = {"X-Internal-Token": self._token} if self._token else None
        try:
            resp = await self._client.post(
                self._url, json=body, headers=headers, timeout=self._timeout
            )
            if resp.status_code >= 400:
                log.warning("audit_emit_failed", status=resp.status_code, action=record.action)
        except httpx.HTTPError as exc:
            log.warning("audit_emit_error", error=str(exc), action=record.action)


def build_audit_sink(settings: Settings, client: httpx.AsyncClient | None = None) -> AuditSink:
    if settings.audit_sink_mode == "noop":
        return NoopAuditSink()
    if settings.audit_sink_mode == "http":
        if client is None:
            log.warning(
                "audit_sink_http_no_client",
                detail="audit_sink_mode=http but no HTTP client provided; using LogAuditSink",
            )
            return LogAuditSink()
        return HttpAuditSink(
            client,
            settings.audit_log_url,
            settings.audit_internal_token,
            settings.audit_emit_timeout_seconds,
        )
    return LogAuditSink()
