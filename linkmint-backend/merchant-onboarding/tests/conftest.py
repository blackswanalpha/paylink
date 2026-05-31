"""Shared fixtures for the unit/API suite (no Docker; in-memory fakes + fakeredis)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.deps import get_idempotency, get_services
from app.domain.services import ServiceDeps, Services, build_services
from app.events.stub import NoopPublisher
from app.idempotency import IdempotencyStore
from app.main import create_app
from tests._support import FakeObjectStore, FakeRepository, make_settings, mint_token, noop_commit


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakeRepository:
    return FakeRepository()


@pytest.fixture
def object_store() -> FakeObjectStore:
    return FakeObjectStore()


@pytest.fixture
def idem_store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakeRepository,
    object_store: FakeObjectStore,
    idem_store: IdempotencyStore,
) -> Iterator[TestClient]:
    """TestClient whose data/redis/object-store deps are in-memory fakes, but which exercises the
    real security primitives (RS256 verify, AES-GCM bank cipher, RBAC, idempotency)."""
    app = create_app(settings)
    with TestClient(app) as test_client:

        async def _services_override() -> AsyncIterator[Services]:
            deps = ServiceDeps(
                repo=fake_repo,  # type: ignore[arg-type]
                commit=noop_commit,
                settings=settings,
                publisher=NoopPublisher(),
                bank_cipher=app.state.bank_cipher,
                object_store=object_store,
            )
            yield build_services(deps)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem_store
        yield test_client
    app.dependency_overrides.clear()


def auth_headers(org_id: str, *, role: str = "owner", user_id: str | None = None) -> dict[str, str]:
    """Bearer header for a principal who is a member of ``org_id`` with ``role``."""
    token = mint_token(user_id=user_id, roles=[(org_id, role)])
    return {"Authorization": f"Bearer {token}"}
