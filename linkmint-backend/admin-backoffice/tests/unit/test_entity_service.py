from __future__ import annotations

import pytest

from app.domain.entity_service import EntityService
from app.errors import AppError, ErrorCode
from tests._support import (
    SAMPLE_MERCHANT_ID,
    SAMPLE_USER_ID,
    CapturingAuditSink,
    fake_registry,
    principal_admin,
)


async def _get(registry, audit, entity_type, entity_id):
    return await EntityService(registry, audit).get(
        principal=principal_admin(),
        scopes=frozenset({"support.read"}),
        ip=None,
        user_agent=None,
        entity_type=entity_type,
        entity_id=entity_id,
    )


async def test_get_ok_and_audits() -> None:
    audit = CapturingAuditSink()
    view = await _get(fake_registry(), audit, "user", SAMPLE_USER_ID)
    assert view.type == "user" and view.data["email"] == "alice@example.com"
    assert audit.records[0].action == "admin.view.user" and audit.records[0].result == "ok"
    assert audit.records[0].resource_id == SAMPLE_USER_ID


async def test_get_not_found_audits_not_found() -> None:
    audit = CapturingAuditSink()
    with pytest.raises(AppError) as exc:
        await _get(fake_registry(), audit, "user", "00000000-0000-0000-0000-000000000000")
    assert exc.value.code == ErrorCode.ENTITY_NOT_FOUND
    assert audit.records[0].result == "not_found"


async def test_get_upstream_error_audits_error() -> None:
    audit = CapturingAuditSink()
    with pytest.raises(AppError) as exc:
        await _get(fake_registry(fail={"merchant"}), audit, "merchant", SAMPLE_MERCHANT_ID)
    assert exc.value.code == ErrorCode.UPSTREAM_UNAVAILABLE
    assert audit.records[0].result == "error"
