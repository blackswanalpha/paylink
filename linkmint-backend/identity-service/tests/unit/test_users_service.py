from __future__ import annotations

import uuid

import pytest

from app.db.models import UserRow
from app.domain.users_service import UsersService
from app.errors import AppError, ErrorCode
from app.events.stub import NoopPublisher
from tests._support import FakeRepository, noop_commit


def _svc(repo: FakeRepository) -> UsersService:
    return UsersService(repo, noop_commit, NoopPublisher())  # type: ignore[arg-type]


async def _seed(repo: FakeRepository, email: str = "u@x.com", phone: str | None = None) -> UserRow:
    user = UserRow(
        user_id=uuid.uuid4(), email=email, phone=phone, password_hash=None, status="ACTIVE"
    )
    await repo.insert_user(user)
    return user


async def test_get_missing_raises() -> None:
    with pytest.raises(AppError) as exc:
        await _svc(FakeRepository()).get(uuid.uuid4())
    assert exc.value.code == ErrorCode.USER_NOT_FOUND


async def test_update_email_and_phone() -> None:
    repo = FakeRepository()
    user = await _seed(repo, "old@x.com")
    updated = await _svc(repo).update(user.user_id, email="new@x.com", phone="+254700000000")
    assert updated.email == "new@x.com"
    assert updated.phone == "+254700000000"


async def test_update_email_conflict() -> None:
    repo = FakeRepository()
    await _seed(repo, "taken@x.com")
    user = await _seed(repo, "me@x.com")
    with pytest.raises(AppError) as exc:
        await _svc(repo).update(user.user_id, email="taken@x.com", phone=None)
    assert exc.value.code == ErrorCode.EMAIL_TAKEN


async def test_update_phone_conflict() -> None:
    repo = FakeRepository()
    await _seed(repo, "a@x.com", phone="+100")
    user = await _seed(repo, "b@x.com")
    with pytest.raises(AppError) as exc:
        await _svc(repo).update(user.user_id, email=None, phone="+100")
    assert exc.value.code == ErrorCode.PHONE_TAKEN


async def test_set_kyc_tier_and_roles() -> None:
    repo = FakeRepository()
    user = await _seed(repo)
    svc = _svc(repo)
    await svc.set_kyc_tier(user.user_id, 2)
    assert (await svc.get(user.user_id)).kyc_tier == 2
    await svc.set_kyc_tier(uuid.uuid4(), 5)  # unknown user → silent no-op
    assert await svc.roles(user.user_id) == []
