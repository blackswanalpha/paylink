"""Refund domain-service logic: eligibility, amount validation, lifecycle, dev simulate."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from app.domain.services import Services
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from tests._support import (
    FakePaylinksClient,
    FakePaymentsClient,
    FakeRefundRepository,
    make_settings,
)
from tests.conftest import build_fake_services


async def _request(services: Services, fake_payments: FakePaymentsClient, **kw):
    fake_payments.add("pay1", paylink_id="0xpl", rail="mpesa", status="SETTLED")
    return await services.refunds.request_refund(
        payment_id="pay1",
        amount_minor=kw.get("amount_minor", 500),
        currency=kw.get("currency", "KES"),
        reason=kw.get("reason"),
        requested_by="user-1",
        org_id=kw.get("org_id"),
        merchant_id=kw.get("merchant_id"),
    )


async def test_request_rejects_unknown_payment(services: Services) -> None:
    with pytest.raises(AppError) as exc:
        await services.refunds.request_refund(
            payment_id="missing",
            amount_minor=100,
            currency="KES",
            reason=None,
            requested_by="u",
            org_id=None,
            merchant_id=None,
        )
    assert exc.value.code == ErrorCode.PAYMENT_NOT_FOUND


async def test_request_rejects_unsettled_payment(
    services: Services, fake_payments: FakePaymentsClient
) -> None:
    fake_payments.add("pay1", status="INITIATED")
    with pytest.raises(AppError) as exc:
        await services.refunds.request_refund(
            payment_id="pay1",
            amount_minor=100,
            currency="KES",
            reason=None,
            requested_by="u",
            org_id=None,
            merchant_id=None,
        )
    assert exc.value.code == ErrorCode.PAYMENT_NOT_SETTLED


async def test_strict_mode_requires_amount_source(
    fake_repo: FakeRefundRepository,
    fake_payments: FakePaymentsClient,
    fake_paylinks: FakePaylinksClient,
) -> None:
    settings = make_settings(amount_validation="strict")
    services = build_fake_services(settings, fake_repo, fake_payments, fake_paylinks)
    fake_payments.add("pay1", paylink_id="0xpl", status="SETTLED")  # no amount source seeded
    with pytest.raises(AppError) as exc:
        await services.refunds.request_refund(
            payment_id="pay1",
            amount_minor=100,
            currency="KES",
            reason=None,
            requested_by="u",
            org_id=None,
            merchant_id=None,
        )
    assert exc.value.code == ErrorCode.AMOUNT_SOURCE_UNAVAILABLE


async def test_partial_flag_and_cumulative_cap(
    services: Services, fake_repo: FakeRefundRepository, fake_payments: FakePaymentsClient
) -> None:
    fake_repo.seed_verified("0xpl", amount_minor=1000)
    r1 = await _request(services, fake_payments, amount_minor=600)
    assert r1.is_partial is True
    # second partial within remaining
    await _request(services, fake_payments, amount_minor=400)
    # now the total is at the cap; a further refund exceeds it
    with pytest.raises(AppError) as exc:
        await _request(services, fake_payments, amount_minor=1)
    assert exc.value.code == ErrorCode.REFUND_EXCEEDS_REMAINING


async def test_full_refund_flag(
    services: Services, fake_repo: FakeRefundRepository, fake_payments: FakePaymentsClient
) -> None:
    fake_repo.seed_verified("0xpl", amount_minor=500)
    r = await _request(services, fake_payments, amount_minor=500)
    assert r.is_partial is False
    assert ev.REFUND_REQUESTED in fake_repo.event_kinds()


async def test_amount_falls_back_to_paylink_service(
    services: Services, fake_payments: FakePaymentsClient, fake_paylinks: FakePaylinksClient
) -> None:
    fake_paylinks.add("0xpl", amount_minor=800)
    r = await _request(services, fake_payments, amount_minor=800)
    assert r.is_partial is False


async def test_approve_instructs_reversal_and_emits(
    services: Services, fake_repo: FakeRefundRepository, fake_payments: FakePaymentsClient
) -> None:
    r = await _request(services, fake_payments, amount_minor=500)
    approved = await services.refunds.approve(r.refund_id, approved_by="admin-1")
    assert approved.state == "PROCESSING"
    assert approved.approved_by == "admin-1"
    assert approved.reversal_ref and approved.reversal_ref.startswith("instruction:mpesa:")
    kinds = fake_repo.event_kinds()
    assert ev.REFUND_APPROVED in kinds
    assert ev.REFUND_REVERSAL_INSTRUCTED in kinds
    assert ev.REFUND_PROCESSING in kinds


async def test_reject(
    services: Services, fake_repo: FakeRefundRepository, fake_payments: FakePaymentsClient
) -> None:
    r = await _request(services, fake_payments)
    rejected = await services.refunds.reject(r.refund_id, rejected_by="admin-1")
    assert rejected.state == "REJECTED"
    assert ev.REFUND_REJECTED in fake_repo.event_kinds()


async def test_illegal_transition(services: Services, fake_payments: FakePaymentsClient) -> None:
    r = await _request(services, fake_payments)
    await services.refunds.reject(r.refund_id, rejected_by="a")
    with pytest.raises(AppError) as exc:
        await services.refunds.approve(r.refund_id, approved_by="a")
    assert exc.value.code == ErrorCode.INVALID_STATE_TRANSITION


async def test_complete_and_fail(
    services: Services, fake_repo: FakeRefundRepository, fake_payments: FakePaymentsClient
) -> None:
    r = await _request(services, fake_payments)
    await services.refunds.approve(r.refund_id, approved_by="a")
    done = await services.refunds.complete(r.refund_id)
    assert done.state == "COMPLETED"
    assert ev.REFUND_COMPLETED in fake_repo.event_kinds()

    r2 = await _request(services, fake_payments)
    await services.refunds.approve(r2.refund_id, approved_by="a")
    failed = await services.refunds.fail(r2.refund_id, reason="rail timeout")
    assert failed.state == "FAILED"
    assert failed.failure_reason == "rail timeout"
    assert ev.REFUND_FAILED in fake_repo.event_kinds()


async def test_get_not_found(services: Services) -> None:
    import uuid

    with pytest.raises(AppError) as exc:
        await services.refunds.get(uuid.uuid4())
    assert exc.value.code == ErrorCode.REFUND_NOT_FOUND


async def test_simulate_due_completions(
    services: Services, fake_payments: FakePaymentsClient
) -> None:
    r = await _request(services, fake_payments)
    await services.refunds.approve(r.refund_id, approved_by="a")
    # push updated_at into the past so it's older than the simulate threshold
    services_row = await services.refunds.get(r.refund_id)
    services_row.updated_at = datetime.now(UTC) - timedelta(seconds=3600)
    n = await services.refunds.simulate_due_completions(datetime.now(UTC))
    assert n == 1
    assert (await services.refunds.get(r.refund_id)).state == "COMPLETED"
