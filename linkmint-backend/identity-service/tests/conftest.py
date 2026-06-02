"""Shared fixtures for the unit/API suite (no Docker; in-memory fakes + fakeredis)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator
from typing import Any

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.deps import get_idempotency, get_services
from app.domain.services import ServiceDeps, Services, build_services
from app.events.stub import NoopPublisher
from app.main import create_app
from app.security.login_attempts import FailedLoginCounter
from tests._support import FakeRepository, make_settings, noop_commit


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakeRepository:
    return FakeRepository()


@pytest.fixture
def idem_store() -> IdempotencyStore:
    return IdempotencyStore(
        fakeredis.aioredis.FakeRedis(decode_responses=True), "identity-service", 3600
    )


@pytest.fixture
def client(
    settings: Settings, fake_repo: FakeRepository, idem_store: IdempotencyStore
) -> Iterator[TestClient]:
    """TestClient whose data/redis dependencies are in-memory fakes, but which exercises the real
    security primitives (argon2, RS256 JWT, RBAC, idempotency)."""
    app = create_app(settings)
    with TestClient(app) as test_client:
        failed_login = FailedLoginCounter(fakeredis.aioredis.FakeRedis(decode_responses=True))

        async def _services_override() -> AsyncIterator[Services]:
            deps = ServiceDeps(
                repo=fake_repo,  # type: ignore[arg-type]
                commit=noop_commit,
                settings=settings,
                publisher=NoopPublisher(),
                passwords=app.state.passwords,
                jwt=app.state.jwt_issuer,
                mfa_cipher=app.state.mfa_cipher,
                oauth=app.state.oauth_resolver,
                failed_login=failed_login,
            )
            yield build_services(deps)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem_store
        yield test_client
    app.dependency_overrides.clear()


def register(
    client: TestClient, email: str = "user@example.com", password: str = "passw0rd123"
) -> str:
    """Register a user and return its user_id."""
    resp = client.post("/v1/auth/register", json={"email": email, "password": password})
    assert resp.status_code == 201, resp.text
    return resp.json()["user_id"]


def login(
    client: TestClient, email: str = "user@example.com", password: str = "passw0rd123", **extra: Any
) -> dict[str, Any]:
    resp = client.post("/v1/auth/login", json={"email": email, "password": password, **extra})
    assert resp.status_code == 200, resp.text
    return resp.json()


def auth_headers(
    client: TestClient, email: str = "user@example.com", password: str = "passw0rd123"
) -> dict[str, str]:
    register(client, email, password)
    tokens = login(client, email, password)
    return {"Authorization": f"Bearer {tokens['access_token']}"}
