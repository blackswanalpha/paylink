"""Shared fixtures for the unit/API suite (no Docker; in-memory fakes + fakeredis)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient
from linkmint_idempotency import IdempotencyStore

from app.config import Settings
from app.deps import get_idempotency, get_services
from app.domain.services import ServiceDeps, Services, build_services
from app.events.stub import NoopPublisher
from app.main import create_app
from tests._support import FakeInvoiceRepository, FakePaylink, make_settings, noop_commit


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakeInvoiceRepository:
    return FakeInvoiceRepository()


@pytest.fixture
def fake_paylink() -> FakePaylink:
    return FakePaylink()


@pytest.fixture
def idem_store() -> IdempotencyStore:
    return IdempotencyStore(
        fakeredis.aioredis.FakeRedis(decode_responses=True), "invoice-subscription", 3600
    )


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakeInvoiceRepository,
    fake_paylink: FakePaylink,
    idem_store: IdempotencyStore,
) -> Iterator[TestClient]:
    """TestClient whose data/redis/paylink deps are in-memory fakes, but which exercises the real
    security primitives (RS256 verify), error handling, idempotency, and HTTP wiring."""
    app = create_app(settings)
    with TestClient(app) as test_client:

        async def _services_override() -> AsyncIterator[Services]:
            deps = ServiceDeps(
                repo=fake_repo,  # type: ignore[arg-type]
                commit=noop_commit,
                settings=settings,
                publisher=NoopPublisher(),
                paylink=fake_paylink,
            )
            yield build_services(deps)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem_store
        yield test_client
    app.dependency_overrides.clear()
