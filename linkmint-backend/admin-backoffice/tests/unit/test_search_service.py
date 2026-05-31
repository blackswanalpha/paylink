from __future__ import annotations

import pytest

from app.domain.search_service import SearchService
from app.errors import AppError, ErrorCode
from tests._support import CapturingAuditSink, fake_registry, principal_admin


def _svc(registry, audit) -> SearchService:
    return SearchService(registry, audit, timeout=2.0, limit=20)


async def test_search_groups_all_types_and_audits_once() -> None:
    audit = CapturingAuditSink()
    res = await _svc(fake_registry(), audit).search(
        principal=principal_admin(),
        scopes=frozenset({"support.read"}),
        ip="1.2.3.4",
        user_agent="ua",
        q="alice",
    )
    assert set(res.groups) == {"user", "merchant", "paylink", "payment"}
    assert len(res.groups["user"]) == 1 and res.groups["user"][0].label == "alice@example.com"
    assert res.degraded == []
    assert len(audit.records) == 1
    rec = audit.records[0]
    assert rec.action == "admin.search" and rec.result == "ok" and rec.query == "alice"
    assert rec.ip == "1.2.3.4"


async def test_one_provider_down_degrades_only_its_group() -> None:
    audit = CapturingAuditSink()
    res = await _svc(fake_registry(fail={"payment"}), audit).search(
        principal=principal_admin(),
        scopes=frozenset(),
        ip=None,
        user_agent=None,
        q="acme",
    )
    assert "payment" in res.degraded
    assert "payment" not in res.groups
    assert "merchant" in res.groups  # the rest still answered (200, partial)
    assert audit.records[0].result == "degraded"


async def test_short_query_rejected() -> None:
    with pytest.raises(AppError) as exc:
        await _svc(fake_registry(), CapturingAuditSink()).search(
            principal=principal_admin(),
            scopes=frozenset(),
            ip=None,
            user_agent=None,
            q="a",
        )
    assert exc.value.code == ErrorCode.INVALID_QUERY


async def test_query_is_trimmed() -> None:
    res = await _svc(fake_registry(), CapturingAuditSink()).search(
        principal=principal_admin(),
        scopes=frozenset(),
        ip=None,
        user_agent=None,
        q="  alice  ",
    )
    assert res.query == "alice"
