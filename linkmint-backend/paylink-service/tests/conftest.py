"""Shared fixtures for the unit/API suite (no Docker needed; uses in-memory fakes)."""

from __future__ import annotations

from typing import Any

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient

from app.chain.nonce import NonceManager
from app.chain.signer import ServiceKeySigner
from app.config import Settings
from app.deps import caller_address, get_idempotency, get_service
from app.domain.service import PayLinkService
from app.events.stub import NoopPublisher
from app.idempotency import IdempotencyStore
from app.main import create_app
from tests._support import (
    GOLDEN_KEY,
    FakeChainClient,
    FakeRepository,
    make_settings,
    noop_commit,
)


@pytest.fixture
def fake_chain() -> FakeChainClient:
    return FakeChainClient()


@pytest.fixture
def fake_repo() -> FakeRepository:
    return FakeRepository()


@pytest.fixture
def idem_store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


@pytest.fixture
def signer() -> ServiceKeySigner:
    return ServiceKeySigner.from_hex(GOLDEN_KEY)


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def service(
    fake_repo: FakeRepository,
    fake_chain: FakeChainClient,
    signer: ServiceKeySigner,
    settings: Settings,
) -> PayLinkService:
    return PayLinkService(
        repo=fake_repo,
        commit=noop_commit,
        chain=fake_chain,  # type: ignore[arg-type]
        signer=signer,
        nonces=NonceManager(fake_chain),  # type: ignore[arg-type]
        publisher=NoopPublisher(),
        settings=settings,
    )


@pytest.fixture
def make_service(
    fake_repo: FakeRepository,
    fake_chain: FakeChainClient,
    signer: ServiceKeySigner,
) -> Any:
    """Build a PayLinkService over the shared fakes with optional settings overrides."""

    def _make(**settings_over: Any) -> PayLinkService:
        return PayLinkService(
            repo=fake_repo,
            commit=noop_commit,
            chain=fake_chain,  # type: ignore[arg-type]
            signer=signer,
            nonces=NonceManager(fake_chain),  # type: ignore[arg-type]
            publisher=NoopPublisher(),
            settings=make_settings(**settings_over),
        )

    return _make


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakeRepository,
    fake_chain: FakeChainClient,
    idem_store: IdempotencyStore,
    signer: ServiceKeySigner,
) -> Any:
    """A TestClient with DB/Redis/chain dependencies overridden by in-memory fakes."""
    app = create_app(settings)
    nonces = NonceManager(fake_chain)  # type: ignore[arg-type]
    publisher = NoopPublisher()

    async def _service_override() -> Any:
        yield PayLinkService(
            repo=fake_repo,
            commit=noop_commit,
            chain=fake_chain,  # type: ignore[arg-type]
            signer=signer,
            nonces=nonces,
            publisher=publisher,
            settings=settings,
        )

    app.dependency_overrides[get_service] = _service_override
    app.dependency_overrides[get_idempotency] = lambda: idem_store
    app.dependency_overrides[caller_address] = lambda: signer.address
    with TestClient(app) as test_client:
        yield test_client
    app.dependency_overrides.clear()
