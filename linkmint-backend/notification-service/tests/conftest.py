"""Shared fixtures for the unit/API suite (no Docker; in-memory fakes + fakeredis)."""

from __future__ import annotations

from collections.abc import Iterator

import fakeredis.aioredis
import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.idempotency import IdempotencyStore
from app.main import create_app
from tests._support import EnqueueSpy, FakeRepository, install_overrides, make_settings


@pytest.fixture
def settings() -> Settings:
    return make_settings()


@pytest.fixture
def fake_repo() -> FakeRepository:
    return FakeRepository()


@pytest.fixture
def enqueue_spy() -> EnqueueSpy:
    return EnqueueSpy()


@pytest.fixture
def idem_store() -> IdempotencyStore:
    return IdempotencyStore(fakeredis.aioredis.FakeRedis(decode_responses=True), 3600)


@pytest.fixture
def client(
    settings: Settings,
    fake_repo: FakeRepository,
    idem_store: IdempotencyStore,
    enqueue_spy: EnqueueSpy,
) -> Iterator[TestClient]:
    app = create_app(settings)
    with TestClient(app) as test_client:
        install_overrides(app, fake_repo, idem_store, enqueue_spy)
        yield test_client
    app.dependency_overrides.clear()
