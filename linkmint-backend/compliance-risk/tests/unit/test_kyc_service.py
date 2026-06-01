from __future__ import annotations

import uuid

import pytest

from app.domain.kyc_service import KycService
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from app.events.stub import NoopPublisher
from app.providers.fake import FakeKycProvider
from app.security.provider_crypto import ProviderCipher
from tests._support import FakeRepository, make_settings, noop_commit


def _svc(repo: FakeRepository) -> KycService:
    settings = make_settings()
    return KycService(
        repo,  # type: ignore[arg-type]
        noop_commit,
        NoopPublisher(),
        ProviderCipher.from_settings(settings),
        settings,
    )


async def test_create_session_upserts_record_without_changing_tier() -> None:
    repo = FakeRepository()
    svc = _svc(repo)
    uid = uuid.uuid4()
    started = await svc.create_session(provider=FakeKycProvider(), user_id=uid, tier_requested=2)
    assert started.session_id
    assert started.provider_url.startswith("https://kyc.stub.local/s/")
    record = repo.kyc[uid]
    assert record.tier == 0  # session does not grant a tier
    assert record.provider == "stub"
    # provider_ref is stored encrypted (not the raw session id).
    assert record.provider_ref is not None and record.provider_ref != started.session_id


async def test_create_session_invalid_tier_rejected() -> None:
    repo = FakeRepository()
    svc = _svc(repo)
    for bad in (0, 3):
        with pytest.raises(AppError) as exc:
            await svc.create_session(
                provider=FakeKycProvider(), user_id=uuid.uuid4(), tier_requested=bad
            )
        assert exc.value.code == ErrorCode.INVALID_TIER


async def test_create_session_already_verified_when_tier_ge_requested() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    svc = _svc(repo)
    with pytest.raises(AppError) as exc:
        await svc.create_session(provider=FakeKycProvider(), user_id=uid, tier_requested=2)
    assert exc.value.code == ErrorCode.ALREADY_VERIFIED


async def test_create_session_allows_upgrade_request() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=1)
    svc = _svc(repo)
    started = await svc.create_session(provider=FakeKycProvider(), user_id=uid, tier_requested=2)
    assert started.session_id
    assert repo.kyc[uid].tier == 1  # still unchanged until a passing callback


async def test_apply_callback_pass_sets_tier_and_emits_user_tier_payload() -> None:
    repo = FakeRepository()
    svc = _svc(repo)
    uid = uuid.uuid4()
    await svc.apply_callback(
        provider=FakeKycProvider(),
        body={"user_id": str(uid), "status": "passed", "tier": 2, "session_id": "sess-1"},
    )
    record = repo.kyc[uid]
    assert record.tier == 2
    assert record.verified_at is not None and record.expires_at is not None
    # documents holds ONLY redacted metadata.
    assert record.documents == {"status": "passed", "session_id": "sess-1", "tier": 2}
    # The emitted event is EXACTLY {user_id, tier} (identity-service's KycConsumer contract).
    kyc_events = [(k, p) for (_, _, k, p) in repo.events if k == ev.KYC_PASSED]
    assert kyc_events == [(ev.KYC_PASSED, {"user_id": str(uid), "tier": 2})]


async def test_apply_callback_pass_never_lowers_tier() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    svc = _svc(repo)
    await svc.apply_callback(
        provider=FakeKycProvider(), body={"user_id": str(uid), "status": "passed", "tier": 1}
    )
    assert repo.kyc[uid].tier == 2  # max(existing, granted)


async def test_apply_callback_fail_emits_user_only_payload() -> None:
    repo = FakeRepository()
    svc = _svc(repo)
    uid = uuid.uuid4()
    await svc.apply_callback(
        provider=FakeKycProvider(), body={"user_id": str(uid), "status": "failed"}
    )
    assert repo.kyc[uid].tier == 0
    fail_events = [(k, p) for (_, _, k, p) in repo.events if k == ev.KYC_FAILED]
    assert fail_events == [(ev.KYC_FAILED, {"user_id": str(uid)})]


async def test_get_status_404_when_unknown() -> None:
    repo = FakeRepository()
    svc = _svc(repo)
    with pytest.raises(AppError) as exc:
        await svc.get_status(uuid.uuid4())
    assert exc.value.code == ErrorCode.COMPLIANCE_NOT_FOUND


async def test_get_status_returns_view() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=1)
    svc = _svc(repo)
    view = await svc.get_status(uid)
    assert view.user_id == str(uid)
    assert view.kyc_tier == 1
    assert view.risk_score is None
    assert view.flags == []
