"""Shared fixtures for the unit/API suite (no Docker; in-memory fakes + fakeredis)."""

from __future__ import annotations

from collections.abc import AsyncIterator, Iterator

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient
from linkmint_idempotency import IdempotencyStore

from app.clawback.coordinator import EventClawbackCoordinator
from app.config import Settings
from app.deps import get_idempotency, get_services
from app.domain.services import ServiceDeps, Services, build_services
from app.events.stub import NoopPublisher
from app.ledger.poster import NoopLedgerPoster
from app.main import create_app
from app.reversal.instruction import InstructionOnlyReversal
from app.reversal.port import RailReversalRegistry
from tests._support import (
    FakePaylinksClient,
    FakePaymentsClient,
    FakeRefundRepository,
    make_settings,
    noop_commit,
)


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakeRefundRepository:
    return FakeRefundRepository()


@pytest.fixture
def fake_payments() -> FakePaymentsClient:
    return FakePaymentsClient()


@pytest.fixture
def fake_paylinks() -> FakePaylinksClient:
    return FakePaylinksClient()


@pytest.fixture
def fake_redis() -> fakeredis.aioredis.FakeRedis:
    return fakeredis.aioredis.FakeRedis(decode_responses=True)


@pytest.fixture
def idem_store(fake_redis: fakeredis.aioredis.FakeRedis) -> IdempotencyStore:
    return IdempotencyStore(fake_redis, "refund-dispute-service", 3600)


def build_fake_services(
    settings: Settings,
    repo: FakeRefundRepository,
    payments: FakePaymentsClient,
    paylinks: FakePaylinksClient,
) -> Services:
    deps = ServiceDeps(
        repo=repo,  # type: ignore[arg-type]
        commit=noop_commit,
        settings=settings,
        publisher=NoopPublisher(),
        payments=payments,
        paylinks=paylinks,
        reversal=RailReversalRegistry(InstructionOnlyReversal()),
        clawback=EventClawbackCoordinator(),
        ledger=NoopLedgerPoster(),
    )
    return build_services(deps)


@pytest.fixture
def services(
    settings: Settings,
    fake_repo: FakeRefundRepository,
    fake_payments: FakePaymentsClient,
    fake_paylinks: FakePaylinksClient,
) -> Services:
    """The domain bundle over fakes — for service-level unit tests without the HTTP layer."""
    return build_fake_services(settings, fake_repo, fake_payments, fake_paylinks)


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakeRefundRepository,
    fake_payments: FakePaymentsClient,
    fake_paylinks: FakePaylinksClient,
    fake_redis: fakeredis.aioredis.FakeRedis,
    idem_store: IdempotencyStore,
) -> Iterator[TestClient]:
    """TestClient whose data/redis/upstream deps are in-memory fakes, but which exercises the real
    security primitives (RS256 verify + HMAC), error handling, idempotency, and HTTP wiring."""
    app = create_app(settings)
    with TestClient(app) as test_client:

        async def _services_override() -> AsyncIterator[Services]:
            yield build_fake_services(settings, fake_repo, fake_payments, fake_paylinks)

        app.dependency_overrides[get_services] = _services_override
        app.dependency_overrides[get_idempotency] = lambda: idem_store
        yield test_client
    app.dependency_overrides.clear()
