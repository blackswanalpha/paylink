"""PayLinkService → notification-service emit (FE work07): created / verified / cancelled, and the
guarantee that a notification failure never fails the PayLink operation."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from app.chain.nonce import NonceManager
from app.domain.models import OffChainStatus
from app.domain.service import CreateCommand, PayLinkService
from app.events.stub import NoopPublisher
from tests._support import make_settings, noop_commit

FUTURE = datetime(2030, 1, 1, tzinfo=UTC)
RECEIVER = "0x0000000000000000000000000000000000000004"


class FakeNotify:
    """Records notify() calls; optionally raises to prove best-effort isolation."""

    def __init__(self, *, raises: bool = False) -> None:
        self.calls: list[dict[str, Any]] = []
        self._raises = raises

    async def notify(
        self,
        *,
        event_kind: str,
        recipient_addr: str,
        data: dict[str, Any],
        dedupe_id: str,
        title: str | None = None,
        body: str | None = None,
        href: str | None = None,
    ) -> None:
        if self._raises:
            raise RuntimeError("notify boom")
        self.calls.append(
            {
                "event_kind": event_kind,
                "recipient_addr": recipient_addr,
                "data": data,
                "dedupe_id": dedupe_id,
                "href": href,
            }
        )

    def kinds(self) -> list[str]:
        return [c["event_kind"] for c in self.calls]


def _svc(fake_repo: Any, fake_chain: Any, signer: Any, notify: Any, **over: Any) -> PayLinkService:
    return PayLinkService(
        repo=fake_repo,
        commit=noop_commit,
        chain=fake_chain,
        signer=signer,
        nonces=NonceManager(fake_chain),
        publisher=NoopPublisher(),
        settings=make_settings(**over),
        notify=notify,
    )


def _cmd(caller: str) -> CreateCommand:
    return CreateCommand(
        receiver=RECEIVER,
        amount=1500,
        currency="PLN",
        expiry=FUTURE,
        usage="single",
        metadata=None,
        rules=None,
        idem_key="idem-1",
        caller_addr=caller,
    )


async def test_create_emits_paylink_created(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    notify = FakeNotify()
    row = await _svc(fake_repo, fake_chain, signer, notify).create(_cmd(signer.address))
    created = [c for c in notify.calls if c["event_kind"] == "paylink.created"]
    assert len(created) == 1
    call = created[0]
    assert call["recipient_addr"] == signer.address
    assert call["data"]["pl_id"] == row.pl_id
    assert call["data"]["amount"] == 1500
    assert call["dedupe_id"] == f"{row.pl_id}:paylink.created"
    assert call["href"] == "/dashboard/paylinks"


async def test_cancel_emits_paylink_cancelled(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    notify = FakeNotify()
    svc = _svc(fake_repo, fake_chain, signer, notify)
    row = await svc.create(_cmd(signer.address))
    await svc.cancel(row.pl_id, signer.address)
    assert "paylink.cancelled" in notify.kinds()


async def test_verified_emits_on_reconcile(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    notify = FakeNotify()
    svc = _svc(fake_repo, fake_chain, signer, notify)
    row = await svc.create(_cmd(signer.address))
    fake_chain.paylinks[row.pl_id] = {
        "id": row.pl_id,
        "creator": signer.address,
        "receiver": RECEIVER,
        "owner": signer.address,
        "amount": 1500,
        "expiry": int(FUTURE.timestamp()),
        "status": "VERIFIED",
        "metadataHash": "0x0",
        "createdAt": 0,
        "voteCount": 3,
    }
    await svc.get(row.pl_id)  # triggers reconcile → VERIFIED
    verified = [c for c in notify.calls if c["event_kind"] == "paylink.verified"]
    assert len(verified) == 1
    assert verified[0]["dedupe_id"] == f"{row.pl_id}:paylink.verified"


async def test_notify_failure_is_non_fatal(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    # A raising notify client must not break create (best-effort isolation).
    svc = _svc(fake_repo, fake_chain, signer, FakeNotify(raises=True))
    row = await svc.create(_cmd(signer.address))
    assert row.status == OffChainStatus.PENDING.value


async def test_no_notify_client_is_safe(fake_repo: Any, fake_chain: Any, signer: Any) -> None:
    # notify=None (disabled) — create proceeds normally with no emit.
    svc = _svc(fake_repo, fake_chain, signer, None)
    row = await svc.create(_cmd(signer.address))
    assert row.status == OffChainStatus.PENDING.value
