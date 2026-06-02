"""NotificationService.intake — fan-out, dedupe, addressable-channel skipping, template guard."""

from __future__ import annotations

import uuid

import pytest

from app.db.models import DeliveryRow
from app.domain.service import NotificationService
from app.errors import AppError, ErrorCode
from app.recipients.inline import InlineRecipientResolver
from app.templating.registry import TemplateRegistry
from tests._support import FakeRepository, noop_commit

DATA = {"amount": "1500", "currency": "KES", "paylink_id": "pl_1", "dedupe_id": "pl_1"}
CONTACT = {"phone": "+254712345678", "email": "jane@example.com"}


def _service(repo: FakeRepository, enqueue: list[object]) -> NotificationService:
    return NotificationService(
        repo=repo,  # type: ignore[arg-type]
        registry=TemplateRegistry(repo),  # type: ignore[arg-type]
        resolver=InlineRecipientResolver(),
        enqueue=enqueue.append,
        commit=noop_commit,
    )


async def test_fan_out_creates_two_deliveries() -> None:
    repo, enqueued = FakeRepository(), []
    ids = await _service(repo, enqueued).intake(
        event_kind="paylink.verified", user_id=uuid.uuid4(), locale="en", data=DATA, contact=CONTACT
    )
    assert len(ids) == 2
    assert len(repo.deliveries) == 2
    assert len(enqueued) == 2
    # Rendered body landed in the payload (placeholders substituted).
    bodies = [row.payload["body"] for row in repo.deliveries.values()]
    assert any("1500 KES" in b for b in bodies)


async def test_dedupe_does_not_double_send() -> None:
    repo, enqueued = FakeRepository(), []
    svc = _service(repo, enqueued)
    uid = uuid.uuid4()
    ids1 = await svc.intake(
        event_kind="paylink.verified", user_id=uid, locale="en", data=DATA, contact=CONTACT
    )
    ids2 = await svc.intake(
        event_kind="paylink.verified", user_id=uid, locale="en", data=DATA, contact=CONTACT
    )
    assert set(ids1) == set(ids2)
    assert len(repo.deliveries) == 2  # not 4
    assert len(enqueued) == 2  # only the first call enqueued


async def test_only_addressable_channels_created() -> None:
    repo, enqueued = FakeRepository(), []
    ids = await _service(repo, enqueued).intake(
        event_kind="paylink.verified",
        user_id=uuid.uuid4(),
        locale="en",
        data=DATA,
        contact={"phone": "+254712345678"},  # SMS only
    )
    assert len(ids) == 1
    assert repo.deliveries[ids[0]].channel == "sms"


async def test_no_contact_creates_nothing() -> None:
    repo, enqueued = FakeRepository(), []
    ids = await _service(repo, enqueued).intake(
        event_kind="paylink.verified", user_id=uuid.uuid4(), locale="en", data=DATA, contact=None
    )
    assert ids == []
    assert len(repo.deliveries) == 0
    assert enqueued == []


async def test_insert_conflict_reuses_existing_no_resend() -> None:
    """A concurrent dedupe race (insert_delivery → None) reuses the winner and does not enqueue."""

    class ConflictRepo(FakeRepository):
        def __init__(self, winner: DeliveryRow) -> None:
            super().__init__()
            self._winner = winner

        async def insert_delivery(self, row: DeliveryRow) -> DeliveryRow | None:
            return None  # always "lost the race"

        async def find_delivery_by_dedupe(self, dedupe_key: str) -> DeliveryRow | None:
            return self._winner

    winner = DeliveryRow(
        delivery_id=uuid.uuid4(),
        channel="sms",
        recipient="+254712345678",
        event_kind="paylink.verified",
        payload={"body": "x", "dedupe_key": "k"},
        status="QUEUED",
        attempts=0,
    )
    enqueued: list[object] = []
    ids = await _service(ConflictRepo(winner), enqueued).intake(
        event_kind="paylink.verified", user_id=uuid.uuid4(), locale="en", data=DATA, contact=CONTACT
    )
    assert ids and all(i == winner.delivery_id for i in ids)
    assert enqueued == []  # nothing newly enqueued — reused existing


async def test_unknown_event_raises_template_not_found() -> None:
    repo, enqueued = FakeRepository(), []
    with pytest.raises(AppError) as exc:
        await _service(repo, enqueued).intake(
            event_kind="ghost.event", user_id=uuid.uuid4(), locale="en", data={}, contact=CONTACT
        )
    assert exc.value.code == ErrorCode.TEMPLATE_NOT_FOUND
