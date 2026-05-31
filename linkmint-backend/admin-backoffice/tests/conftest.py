"""Shared fixtures for the unit/API suite (no Docker; in-memory providers + fake staff repo)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator

import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.deps import get_admin_repo, get_app_services
from app.domain.services import build_services
from app.main import create_app
from tests._support import (
    CapturingAuditSink,
    FakeAdminRepository,
    ProviderRegistry,
    fake_registry,
    make_settings,
)


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def audit() -> CapturingAuditSink:
    return CapturingAuditSink()


@pytest.fixture
def admin_repo() -> FakeAdminRepository:
    return FakeAdminRepository()


@pytest.fixture
def registry() -> ProviderRegistry:
    return fake_registry()


@pytest.fixture
def client(
    settings: Settings,
    registry: ProviderRegistry,
    audit: CapturingAuditSink,
    admin_repo: FakeAdminRepository,
) -> Iterator[TestClient]:
    """TestClient whose providers/audit/staff-repo are in-memory fakes, but which exercises the real
    security primitives (RS256 verify, admin+MFA gate, default-deny scopes)."""
    app = create_app(settings)
    with TestClient(app) as test_client:
        services = build_services(registry, audit, settings)  # type: ignore[arg-type]
        app.dependency_overrides[get_app_services] = lambda: services

        async def _admin_repo_override() -> AsyncIterator[FakeAdminRepository]:
            yield admin_repo

        app.dependency_overrides[get_admin_repo] = _admin_repo_override
        yield test_client
    app.dependency_overrides.clear()
