from __future__ import annotations

from app.domain.models import SCOPES
from app.domain.scopes import ScopeResolver
from tests._support import FakeAdminRepository, make_settings


async def test_default_deny_for_unknown_sub() -> None:
    resolver = ScopeResolver(FakeAdminRepository(), make_settings())  # type: ignore[arg-type]
    assert await resolver.resolve("nobody") == frozenset()


async def test_db_grant() -> None:
    repo = FakeAdminRepository()
    repo.grant("u1", ["support.read"])
    resolver = ScopeResolver(repo, make_settings())  # type: ignore[arg-type]
    assert "support.read" in await resolver.resolve("u1")


async def test_env_seed_grant() -> None:
    settings = make_settings(dev_staff_grants="u2:support.read,finance.refund; bad-entry")
    resolver = ScopeResolver(FakeAdminRepository(), settings)  # type: ignore[arg-type]
    granted = await resolver.resolve("u2")
    assert {"support.read", "finance.refund"} <= granted


async def test_superuser_expands_to_all() -> None:
    repo = FakeAdminRepository()
    repo.grant("god", ["superuser"])
    resolver = ScopeResolver(repo, make_settings())  # type: ignore[arg-type]
    assert await resolver.resolve("god") == SCOPES


async def test_db_and_env_union() -> None:
    repo = FakeAdminRepository()
    repo.grant("u3", ["support.read"])
    settings = make_settings(dev_staff_grants="u3:finance.refund")
    resolver = ScopeResolver(repo, settings)  # type: ignore[arg-type]
    assert {"support.read", "finance.refund"} <= await resolver.resolve("u3")
