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
from app.ledger.poster import NoopLedgerPoster
from app.main import create_app
from tests._support import FakeFxProvider, FakePricingRepository, make_settings, noop_commit


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakePricingRepository:
    return FakePricingRepository()


@pytest.fixture
def fake_fx() -> FakeFxProvider:
    return FakeFxProvider()


@pytest.fixture
def fake_redis() -> fakeredis.aioredis.FakeRedis:
    return fakeredis.aioredis.FakeRedis(decode_responses=True)


@pytest.fixture
def idem_store(fake_redis: fakeredis.aioredis.FakeRedis) -> IdempotencyStore:
    return IdempotencyStore(fake_redis, "fee-pricing-service", 3600)


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakePricingRepository,
    fake_fx: FakeFxProvider,
    fake_redis: fakeredis.aioredis.FakeRedis,
    idem_store: IdempotencyStore,
) -> Iterator[TestClient]:
    """TestClient whose data/redis/fx deps are in-memory fakes, but which exercises the real
    security primitives (RS256 verify), error handling, idempotency, and HTTP wiring."""
    app = create_app(settings)
    with TestClient(app) as test_client:

        async def _services_override() -> AsyncIterator[Services]:
            deps = ServiceDeps(
                repo=fake_repo,  # type: ignore[arg-type]
                commit=noop_commit,
                settings=settings,
                publisher=NoopPublisher(),
                fx_provider=fake_fx,
                redis=fake_redis,
                ledger=NoopLedgerPoster(),
            )
            yield build_services(deps)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem_store
        yield test_client
    app.dependency_overrides.clear()
