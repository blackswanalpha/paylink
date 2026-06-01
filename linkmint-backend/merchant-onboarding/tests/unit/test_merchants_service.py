from __future__ import annotations

import uuid

import pytest

from app.config import Settings
from app.domain.bank_accounts_service import BankAccountsService
from app.domain.contracts_service import ContractsService
from app.domain.merchants_service import MerchantsService
from app.domain.models import BankAccountStatus, MerchantStatus, ReviewDecision
from app.errors import AppError, ErrorCode
from app.events.publisher import (
    BANK_ACCOUNT_ADDED,
    CONTRACT_ACCEPTED,
    MERCHANT_ONBOARDED,
    MERCHANT_VERIFIED,
)
from app.events.stub import NoopPublisher
from app.security.bank_crypto import BankCipher
from app.security.jwt import AccessClaims, OrgRole
from tests._support import FakeRepository, make_settings, noop_commit

ORG = str(uuid.uuid4())


def _principal(org_id: str = ORG, role: str = "owner") -> AccessClaims:
    return AccessClaims(
        user_id=str(uuid.uuid4()),
        roles=[OrgRole(org_id=org_id, role=role)],
        user_roles=["payer"],
        kyc_tier=1,
        jti="j",
        sid="s",
    )


def _merchants(repo: FakeRepository, settings: Settings | None = None) -> MerchantsService:
    return MerchantsService(repo, noop_commit, NoopPublisher(), settings or make_settings())


def _banks(repo: FakeRepository, settings: Settings | None = None) -> BankAccountsService:
    cipher = BankCipher.from_settings(settings or make_settings())
    return BankAccountsService(repo, noop_commit, cipher, NoopPublisher())


def _contracts(repo: FakeRepository) -> ContractsService:
    return ContractsService(repo, noop_commit, NoopPublisher())


async def _onboard(
    repo: FakeRepository, principal: AccessClaims, settings: Settings | None = None
) -> uuid.UUID:
    merchant = await _merchants(repo, settings).onboard(
        principal=principal,
        org_id=uuid.UUID(principal.roles[0].org_id),
        business_name="Acme Ltd",
        registration_no="C.12345",
        country="KE",
        merchant_type="company",
    )
    return merchant.merchant_id


# ── onboard ──
async def test_onboard_creates_pending_and_emits() -> None:
    repo = FakeRepository()
    p = _principal()
    merchant = await _merchants(repo).onboard(
        principal=p,
        org_id=uuid.UUID(ORG),
        business_name="Acme",
        registration_no=None,
        country="ke",  # case-insensitive
        merchant_type="company",
    )
    assert merchant.status == MerchantStatus.PENDING_VERIFICATION.value
    assert merchant.country == "KE"
    assert any(k == MERCHANT_ONBOARDED for (_, _, k, _) in repo.events)


async def test_onboard_duplicate_org_is_already_onboarded() -> None:
    repo = FakeRepository()
    p = _principal()
    await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _onboard(repo, p)
    assert exc.value.code == ErrorCode.ALREADY_ONBOARDED


async def test_onboard_non_member_is_org_not_found() -> None:
    repo = FakeRepository()
    p = _principal(role="owner")  # member of ORG…
    other_org = uuid.uuid4()
    with pytest.raises(AppError) as exc:
        await _merchants(repo).onboard(
            principal=p,
            org_id=other_org,  # …but onboarding a different org
            business_name="X",
            registration_no=None,
            country="KE",
            merchant_type="company",
        )
    assert exc.value.code == ErrorCode.ORG_NOT_FOUND


async def test_onboard_unsupported_country() -> None:
    repo = FakeRepository()
    with pytest.raises(AppError) as exc:
        await _merchants(repo).onboard(
            principal=_principal(),
            org_id=uuid.UUID(ORG),
            business_name="X",
            registration_no=None,
            country="US",
            merchant_type="company",
        )
    assert exc.value.code == ErrorCode.UNSUPPORTED_COUNTRY


async def test_onboard_invalid_type() -> None:
    repo = FakeRepository()
    with pytest.raises(AppError) as exc:
        await _merchants(repo).onboard(
            principal=_principal(),
            org_id=uuid.UUID(ORG),
            business_name="X",
            registration_no=None,
            country="KE",
            merchant_type="bogus",
        )
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


# ── get / fee tier ──
async def test_get_for_member_not_found_for_outsider() -> None:
    repo = FakeRepository()
    mid = await _onboard(repo, _principal())
    outsider = _principal(org_id=str(uuid.uuid4()))
    with pytest.raises(AppError) as exc:
        await _merchants(repo).get_for_member(principal=outsider, merchant_id=mid)
    assert exc.value.code == ErrorCode.MERCHANT_NOT_FOUND


async def test_set_fee_tier_requires_admin() -> None:
    repo = FakeRepository()
    mid = await _onboard(repo, _principal())
    viewer = _principal(role="viewer")
    with pytest.raises(AppError) as exc:
        await _merchants(repo).set_fee_tier(principal=viewer, merchant_id=mid, tier="enterprise")
    assert exc.value.code == ErrorCode.FORBIDDEN


async def test_set_fee_tier_ok() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    merchant = await _merchants(repo).set_fee_tier(principal=p, merchant_id=mid, tier="enterprise")
    assert merchant.fee_tier == "enterprise"


async def test_set_fee_tier_invalid() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _merchants(repo).set_fee_tier(principal=p, merchant_id=mid, tier="platinum")
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


# ── decide / activation preconditions ──
async def test_decide_approve_blocked_without_bank_and_contract() -> None:
    repo = FakeRepository()
    mid = await _onboard(repo, _principal())
    with pytest.raises(AppError) as exc:
        await _merchants(repo).decide(merchant_id=mid, decision=ReviewDecision.APPROVE)
    assert exc.value.code == ErrorCode.INVALID_TRANSITION
    assert exc.value.details["missing"] == "verified_bank_account"


async def test_decide_approve_blocked_without_contract() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    bank = await _banks(repo).add_bank_account(
        principal=p,
        merchant_id=mid,
        rail="mpesa",
        account_details="254700123456",
        currency="kes",
        country="KE",
    )
    await _banks(repo).verify_bank_account(
        principal=p, merchant_id=mid, bank_account_id=bank.bank_account_id
    )
    with pytest.raises(AppError) as exc:
        await _merchants(repo).decide(merchant_id=mid, decision=ReviewDecision.APPROVE)
    assert exc.value.details["missing"] == "accepted_contract"


async def test_decide_approve_succeeds_with_preconditions_met() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    bank = await _banks(repo).add_bank_account(
        principal=p,
        merchant_id=mid,
        rail="mpesa",
        account_details="254700123456",
        currency="KES",
        country="KE",
    )
    await _banks(repo).verify_bank_account(
        principal=p, merchant_id=mid, bank_account_id=bank.bank_account_id
    )
    await _contracts(repo).accept_contract(
        principal=p, merchant_id=mid, version="v1", accepted=True, ip=None, user_agent=None
    )
    merchant = await _merchants(repo).decide(merchant_id=mid, decision=ReviewDecision.APPROVE)
    assert merchant.status == MerchantStatus.ACTIVE.value
    assert merchant.onboarded_at is not None
    assert any(k == MERCHANT_VERIFIED for (_, _, k, _) in repo.events)


async def test_decide_bare_state_machine_when_preconditions_disabled() -> None:
    repo = FakeRepository()
    settings = make_settings(
        require_verified_bank_for_active=False, require_contract_for_active=False
    )
    p = _principal()
    mid = await _onboard(repo, p, settings)
    merchant = await _merchants(repo, settings).decide(
        merchant_id=mid, decision=ReviewDecision.APPROVE
    )
    assert merchant.status == MerchantStatus.ACTIVE.value


async def test_decide_reject_then_terminal() -> None:
    repo = FakeRepository()
    mid = await _onboard(repo, _principal())
    m = await _merchants(repo).decide(
        merchant_id=mid, decision=ReviewDecision.REJECT, reason="bad docs"
    )
    assert m.status == MerchantStatus.REJECTED.value
    # REJECTED is terminal — any further decision is INVALID_TRANSITION.
    with pytest.raises(AppError) as exc:
        await _merchants(repo).decide(merchant_id=mid, decision=ReviewDecision.REINSTATE)
    assert exc.value.code == ErrorCode.INVALID_TRANSITION


async def test_decide_suspend_then_reinstate() -> None:
    repo = FakeRepository()
    settings = make_settings(
        require_verified_bank_for_active=False, require_contract_for_active=False
    )
    p = _principal()
    mid = await _onboard(repo, p, settings)
    svc = _merchants(repo, settings)
    await svc.decide(merchant_id=mid, decision=ReviewDecision.APPROVE)
    suspended = await svc.decide(
        merchant_id=mid, decision=ReviewDecision.SUSPEND, reason="risk review"
    )
    assert suspended.status == MerchantStatus.SUSPENDED.value
    assert suspended.suspended_reason == "risk review"
    reinstated = await svc.decide(merchant_id=mid, decision=ReviewDecision.REINSTATE)
    assert reinstated.status == MerchantStatus.ACTIVE.value
    assert reinstated.suspended_reason is None


async def test_decide_unknown_merchant() -> None:
    repo = FakeRepository()
    with pytest.raises(AppError) as exc:
        await _merchants(repo).decide(merchant_id=uuid.uuid4(), decision=ReviewDecision.APPROVE)
    assert exc.value.code == ErrorCode.MERCHANT_NOT_FOUND


# ── bank accounts ──
async def test_add_bank_account_encrypts_ref_no_plaintext() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    secret = "GB29NWBK60161331926819"
    account = await _banks(repo).add_bank_account(
        principal=p,
        merchant_id=mid,
        rail="swift",
        account_details=secret,
        currency="GBP",
        country="KE",
    )
    assert account.status == BankAccountStatus.PENDING_VERIFY.value
    # The stored ref is ciphertext — the plaintext must not appear in it.
    assert secret not in account.account_ref
    # The emitted event payload carries no plaintext / ref either.
    added = [pl for (_, _, k, pl) in repo.events if k == BANK_ACCOUNT_ADDED][0]
    assert secret not in str(added)
    assert "account_ref" not in added and "account_details" not in added


async def test_add_bank_account_invalid_rail() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _banks(repo).add_bank_account(
            principal=p,
            merchant_id=mid,
            rail="paypal",
            account_details="x",
            currency="USD",
            country="KE",
        )
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


async def test_add_bank_account_invalid_account_details() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _banks(repo).add_bank_account(
            principal=p,
            merchant_id=mid,
            rail="mpesa",
            account_details="   ",  # blank after strip → INVALID_ACCOUNT
            currency="KES",
            country="KE",
        )
    assert exc.value.code == ErrorCode.INVALID_ACCOUNT


async def test_verify_bank_account_twice_is_conflict() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    bank = await _banks(repo).add_bank_account(
        principal=p,
        merchant_id=mid,
        rail="mpesa",
        account_details="254700123456",
        currency="KES",
        country="KE",
    )
    await _banks(repo).verify_bank_account(
        principal=p, merchant_id=mid, bank_account_id=bank.bank_account_id
    )
    with pytest.raises(AppError) as exc:
        await _banks(repo).verify_bank_account(
            principal=p, merchant_id=mid, bank_account_id=bank.bank_account_id
        )
    assert exc.value.code == ErrorCode.INVALID_TRANSITION


async def test_verify_unknown_bank_account() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _banks(repo).verify_bank_account(
            principal=p, merchant_id=mid, bank_account_id=uuid.uuid4()
        )
    assert exc.value.code == ErrorCode.BANK_ACCOUNT_NOT_FOUND


# ── contracts ──
async def test_accept_contract_requires_true() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    with pytest.raises(AppError) as exc:
        await _contracts(repo).accept_contract(
            principal=p, merchant_id=mid, version="v1", accepted=False, ip=None, user_agent=None
        )
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


async def test_accept_and_list_contracts() -> None:
    repo = FakeRepository()
    p = _principal()
    mid = await _onboard(repo, p)
    await _contracts(repo).accept_contract(
        principal=p,
        merchant_id=mid,
        version="v1",
        accepted=True,
        ip="10.0.0.5",
        user_agent="pytest",
    )
    items = await _contracts(repo).list_contracts(principal=p, merchant_id=mid)
    assert len(items) == 1 and items[0].version == "v1"
    assert any(k == CONTRACT_ACCEPTED for (_, _, k, _) in repo.events)
