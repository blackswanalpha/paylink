"""Dispute domain-service logic: intake, evidence window, submit, resolution, expiry, clawback."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import pytest

from app.domain.services import Services
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from tests._support import FakeRefundRepository


def _open_body(pid: str = "d-1", **kw) -> dict:
    body = {
        "kind": "dispute.opened",
        "provider_dispute_id": pid,
        "payment_id": "pay-1",
        "rail": "card",
        "amount_minor": 1000,
        "currency": "KES",
        "reason_code": "fraud",
    }
    body.update(kw)
    return body


async def _open(services: Services, **kw) -> str:
    res = await services.disputes.intake(provider="stub", body=_open_body(**kw))
    assert res.action == "opened"
    assert res.dispute_id
    return res.dispute_id


async def test_open_creates_and_emits(services: Services, fake_repo: FakeRefundRepository) -> None:
    did = await _open(services)
    row = fake_repo.disputes[uuid.UUID(did)]
    assert row.state == "OPEN"
    assert ev.DISPUTE_OPENED in fake_repo.event_kinds()


async def test_open_replay_is_noop(services: Services, fake_repo: FakeRefundRepository) -> None:
    await _open(services, pid="dup")
    res = await services.disputes.intake(provider="stub", body=_open_body(pid="dup"))
    assert res.action == "opened_replay"
    assert len(fake_repo.disputes) == 1


async def test_open_uses_window_default(
    services: Services, fake_repo: FakeRefundRepository
) -> None:
    did = await _open(services)
    row = fake_repo.disputes[uuid.UUID(did)]
    # default window is 168h ≈ 7 days from now
    assert row.evidence_due_at > datetime.now(UTC) + timedelta(days=6)


async def test_open_missing_id_raises(services: Services) -> None:
    with pytest.raises(AppError) as exc:
        await services.disputes.intake(provider="stub", body={"kind": "dispute.opened"})
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


async def test_unknown_kind_ignored(services: Services) -> None:
    res = await services.disputes.intake(provider="stub", body={"kind": "weird"})
    assert res.action == "ignored"


async def test_resolution_lost_requests_clawback(
    services: Services, fake_repo: FakeRefundRepository
) -> None:
    did = await _open(services, pid="loss")
    res = await services.disputes.intake(
        provider="stub",
        body={"kind": "dispute.resolved", "provider_dispute_id": "loss", "outcome": "lost"},
    )
    assert res.action == "resolved"
    row = fake_repo.disputes[uuid.UUID(did)]
    assert row.state == "LOST"
    assert row.clawback_requested is True
    assert ev.DISPUTE_LOST in fake_repo.event_kinds()
    payload = fake_repo.event_payload(ev.REFUND_CLAWBACK_REQUESTED)
    assert payload["dispute_id"] == did
    assert payload["reason"] == "dispute_lost"
    assert payload["amount_minor"] == 1000


async def test_resolution_won_no_clawback(
    services: Services, fake_repo: FakeRefundRepository
) -> None:
    await _open(services, pid="win")
    await services.disputes.intake(
        provider="stub",
        body={"kind": "dispute.resolved", "provider_dispute_id": "win", "outcome": "won"},
    )
    assert ev.DISPUTE_WON in fake_repo.event_kinds()
    assert ev.REFUND_CLAWBACK_REQUESTED not in fake_repo.event_kinds()


async def test_resolution_idempotent(services: Services, fake_repo: FakeRefundRepository) -> None:
    await _open(services, pid="x")
    await services.disputes.intake(
        provider="stub",
        body={"kind": "dispute.resolved", "provider_dispute_id": "x", "outcome": "lost"},
    )
    res = await services.disputes.intake(
        provider="stub",
        body={"kind": "dispute.resolved", "provider_dispute_id": "x", "outcome": "lost"},
    )
    assert res.action == "resolved_noop"
    # only one clawback emitted
    assert fake_repo.event_kinds().count(ev.REFUND_CLAWBACK_REQUESTED) == 1


async def test_resolution_unknown_dispute(services: Services) -> None:
    with pytest.raises(AppError) as exc:
        await services.disputes.intake(
            provider="stub",
            body={"kind": "dispute.resolved", "provider_dispute_id": "nope", "outcome": "won"},
        )
    assert exc.value.code == ErrorCode.DISPUTE_NOT_FOUND


async def test_resolution_bad_outcome(services: Services) -> None:
    await _open(services, pid="b")
    with pytest.raises(AppError) as exc:
        await services.disputes.intake(
            provider="stub",
            body={"kind": "dispute.resolved", "provider_dispute_id": "b", "outcome": "maybe"},
        )
    assert exc.value.code == ErrorCode.INVALID_PAYLOAD


async def test_evidence_within_window(services: Services, fake_repo: FakeRefundRepository) -> None:
    did = await _open(services)
    ev_row = await services.disputes.add_evidence(
        dispute_id=uuid.UUID(did),
        kind="receipt",
        summary="proof of delivery",
        payload={"url": "x"},
        external_ref="ref-1",
        submitted_by="user-1",
    )
    assert ev_row.kind == "receipt"
    assert ev.DISPUTE_EVIDENCE_ADDED in fake_repo.event_kinds()


async def test_evidence_window_closed(services: Services, fake_repo: FakeRefundRepository) -> None:
    did = await _open(services)
    fake_repo.disputes[uuid.UUID(did)].evidence_due_at = datetime.now(UTC) - timedelta(hours=1)
    with pytest.raises(AppError) as exc:
        await services.disputes.add_evidence(
            dispute_id=uuid.UUID(did),
            kind="receipt",
            summary=None,
            payload={},
            external_ref=None,
            submitted_by="u",
        )
    assert exc.value.code == ErrorCode.EVIDENCE_WINDOW_CLOSED


async def test_submit(services: Services, fake_repo: FakeRefundRepository) -> None:
    did = await _open(services)
    await services.disputes.add_evidence(
        dispute_id=uuid.UUID(did),
        kind="note",
        summary=None,
        payload={},
        external_ref=None,
        submitted_by="u",
    )
    row = await services.disputes.submit(uuid.UUID(did), submitted_by="admin-1")
    assert row.state == "SUBMITTED"
    assert row.submitted_at is not None
    assert ev.DISPUTE_SUBMITTED in fake_repo.event_kinds()


async def test_submit_then_resolve(services: Services, fake_repo: FakeRefundRepository) -> None:
    did = await _open(services, pid="sr")
    await services.disputes.submit(uuid.UUID(did), submitted_by="a")
    await services.disputes.intake(
        provider="stub",
        body={"kind": "dispute.resolved", "provider_dispute_id": "sr", "outcome": "won"},
    )
    assert fake_repo.disputes[uuid.UUID(did)].state == "WON"


async def test_expire_due_emits_clawback(
    services: Services, fake_repo: FakeRefundRepository
) -> None:
    did = await _open(services, pid="exp")
    fake_repo.disputes[uuid.UUID(did)].evidence_due_at = datetime.now(UTC) - timedelta(hours=1)
    n = await services.disputes.expire_due(datetime.now(UTC))
    assert n == 1
    row = fake_repo.disputes[uuid.UUID(did)]
    assert row.state == "EXPIRED"
    assert row.clawback_requested is True
    assert ev.DISPUTE_EXPIRED in fake_repo.event_kinds()
    assert fake_repo.event_payload(ev.REFUND_CLAWBACK_REQUESTED)["reason"] == "dispute_expired"


async def test_get_not_found(services: Services) -> None:
    with pytest.raises(AppError) as exc:
        await services.disputes.get(uuid.uuid4())
    assert exc.value.code == ErrorCode.DISPUTE_NOT_FOUND
