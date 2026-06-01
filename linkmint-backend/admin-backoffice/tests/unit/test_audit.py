from __future__ import annotations

import json

import httpx

from app.audit.sink import (
    AuditRecord,
    HttpAuditSink,
    LogAuditSink,
    NoopAuditSink,
    build_audit_sink,
)
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


def test_build_http_without_client_falls_back_to_log() -> None:
    # http mode but no HTTP client provided → safe fallback (never silently inert).
    assert isinstance(build_audit_sink(make_settings(audit_sink_mode="http")), LogAuditSink)


async def test_build_http_with_client_selects_http() -> None:
    async with httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _: httpx.Response(201))
    ) as client:
        sink = build_audit_sink(
            make_settings(
                audit_sink_mode="http",
                audit_log_url="http://audit:8094",
                audit_internal_token="tok",
            ),
            client,
        )
        assert isinstance(sink, HttpAuditSink)


async def test_http_sink_maps_record_to_intake_shape() -> None:
    captured: dict[str, object] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        captured["token"] = request.headers.get("X-Internal-Token")
        captured["body"] = json.loads(request.content)
        return httpx.Response(201, json={"entry_id": 1, "hash": "ab"})

    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        sink = HttpAuditSink(client, "http://audit:8094", "tok", 2.0)
        await sink.emit(
            AuditRecord(
                actor="11111111-1111-1111-1111-111111111111",
                actor_scopes=["support.read"],
                action="admin.view.user",
                resource_type="user",
                resource_id="abc",
                query=None,
                result="ok",
                trace_id="t1",
                ip="1.2.3.4",
                user_agent="ua",
            )
        )

    assert str(captured["url"]).endswith("/v1/audit-log")
    assert captured["token"] == "tok"
    body = captured["body"]
    assert isinstance(body, dict)
    assert body["actor"] == "11111111-1111-1111-1111-111111111111"  # bare-string sub
    assert body["resource"] == "user:abc"
    assert body["action"] == "admin.view.user"
    assert body["context"]["trace_id"] == "t1"
    assert body["context"]["result"] == "ok"
    assert body["context"]["actor_scopes"] == ["support.read"]


async def test_http_sink_swallows_error_status() -> None:
    async with httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _: httpx.Response(503))
    ) as client:
        # A 5xx from audit-log-service must not raise (best-effort).
        await HttpAuditSink(client, "http://audit:8094", None, 2.0).emit(_record())


async def test_http_sink_swallows_transport_error() -> None:
    def boom(_: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("audit-log-service is down")

    async with httpx.AsyncClient(transport=httpx.MockTransport(boom)) as client:
        # A transport failure (outage) must not raise — an admin read is never broken by audit.
        await HttpAuditSink(client, "http://audit:8094", None, 2.0).emit(_record())
