from __future__ import annotations

from app.audit.sink import AuditRecord, LogAuditSink, NoopAuditSink, build_audit_sink
from tests._support import make_settings


def _record() -> AuditRecord:
    return AuditRecord(
        actor="u",
        actor_scopes=["support.read"],
        action="admin.search",
        resource_type="search",
        resource_id=None,
        query="q",
        result="ok",
        trace_id="t",
        ip="1.2.3.4",
        user_agent="ua",
    )


async def test_log_sink_emits_without_error() -> None:
    await LogAuditSink().emit(_record())  # exercises the structured-log line


async def test_noop_sink_is_inert() -> None:
    await NoopAuditSink().emit(_record())


def test_build_audit_sink_selects_impl() -> None:
    assert isinstance(build_audit_sink(make_settings(audit_sink_mode="noop")), NoopAuditSink)
    assert isinstance(build_audit_sink(make_settings(audit_sink_mode="log")), LogAuditSink)
