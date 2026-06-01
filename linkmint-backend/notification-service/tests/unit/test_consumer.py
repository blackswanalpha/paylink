"""The event consumer chokepoint — routing + the unknown/bad → no-op contract."""

from __future__ import annotations

import uuid

from app.domain.service import NotificationService
from app.events.consumer import NotificationEventConsumer
from app.recipients.inline import InlineRecipientResolver
from app.templating.registry import TemplateRegistry
from tests._support import FakeRepository, noop_commit

CONTACT = {"phone": "+254712345678", "email": "jane@example.com"}
DATA = {"amount": "1500", "currency": "KES", "paylink_id": "pl_1"}


def _consumer(repo: FakeRepository, enqueue: list[object]) -> NotificationEventConsumer:
    service = NotificationService(
        repo=repo,  # type: ignore[arg-type]
        registry=TemplateRegistry(repo),  # type: ignore[arg-type]
        resolver=InlineRecipientResolver(),
        enqueue=enqueue.append,
        commit=noop_commit,
    )
    return NotificationEventConsumer(service)


async def test_paylink_verified_fans_out_sms_and_email() -> None:
    repo, enqueued = FakeRepository(), []
    ids = await _consumer(repo, enqueued).handle(
        "paylink.verified",
        {"user_id": str(uuid.uuid4()), "data": DATA, "contact": CONTACT},
    )
    assert len(ids) == 2
    assert len(enqueued) == 2


async def test_payment_failed_supported() -> None:
    repo, enqueued = FakeRepository(), []
    ids = await _consumer(repo, enqueued).handle(
        "payment.failed",
        {
            "user_id": str(uuid.uuid4()),
            "data": {"amount": "10", "currency": "KES", "reason": "x"},
            "contact": CONTACT,
        },
    )
    assert len(ids) == 2


async def test_unknown_event_is_noop() -> None:
    assert (
        await _consumer(FakeRepository(), []).handle(
            "mystery.thing", {"user_id": str(uuid.uuid4())}
        )
        == []
    )


async def test_missing_user_is_noop() -> None:
    assert await _consumer(FakeRepository(), []).handle("paylink.verified", {"data": {}}) == []


async def test_bad_user_is_noop() -> None:
    assert (
        await _consumer(FakeRepository(), []).handle("paylink.verified", {"user_id": "not-a-uuid"})
        == []
    )
