from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

import pytest

from app.domain.models import OffChainStatus
from app.domain.service import CreateCommand, PayLinkService
from app.errors import AppError, ErrorCode
from app.events.publisher import PAYLINK_CANCELLED, PAYLINK_CREATED, PAYLINK_REQUESTED

FUTURE = datetime(2030, 1, 1, tzinfo=UTC)
RECEIVER = "0x0000000000000000000000000000000000000004"


def _cmd(caller: str, **over: Any) -> CreateCommand:
    base: dict[str, Any] = {
        "receiver": RECEIVER,
        "amount": 1500,
        "currency": "PLN",
        "expiry": FUTURE,
        "usage": "single",
        "metadata": {"orderId": "INV1"},
        "rules": None,
        "idem_key": "idem-1",
        "caller_addr": caller,
    }
    base.update(over)
    return CreateCommand(**base)


async def test_create_persists_and_submits(
    service: PayLinkService, fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    row = await service.create(_cmd(signer.address))
    assert row.status == OffChainStatus.PENDING.value
    assert row.chain_tx_hash is not None
    assert row.creator_addr == signer.address
    assert row.owner_addr == signer.address
    assert len(fake_chain.sent) == 1
    assert fake_chain.sent[0]["type"] == 1  # TxCreatePayLink
    kinds = [k for _, k, _ in fake_repo.events]
    assert PAYLINK_REQUESTED in kinds
    assert PAYLINK_CREATED in kinds


async def test_only_metadata_hash_goes_on_chain(
    service: PayLinkService, fake_chain: Any, signer: Any
) -> None:
    # Invariant A.1: raw metadata never crosses to the chain — only a hash.
    await service.create(_cmd(signer.address, metadata={"secret": "do-not-leak"}))
    payload = fake_chain.sent[0]["payload"]
    assert "metadataHash" in payload
    assert "metadata" not in payload
    assert "do-not-leak" not in str(payload)


async def test_create_without_submit_stays_created(
    make_service: Any, fake_chain: Any, signer: Any
) -> None:
    svc = make_service(chain_submit_enabled=False)
    row = await svc.create(_cmd(signer.address))
    assert row.status == OffChainStatus.CREATED.value
    assert row.chain_tx_hash is None
    assert fake_chain.sent == []


async def test_create_chain_failure_keeps_record_and_raises(
    make_service: Any, fake_repo: Any, fake_chain: Any, signer: Any
) -> None:
    fake_chain.fail_send = True
    svc = make_service()
    with pytest.raises(AppError) as exc:
        await svc.create(_cmd(signer.address))
    assert exc.value.code is ErrorCode.CHAIN_UNAVAILABLE
    # the CREATED record was persisted before the (failed) submit
    rows = list(fake_repo.rows.values())
    assert len(rows) == 1
    assert rows[0].status == OffChainStatus.CREATED.value


async def test_get_missing_raises_not_found(service: PayLinkService) -> None:
    with pytest.raises(AppError) as exc:
        await service.get("0xmissing")
    assert exc.value.code is ErrorCode.PAYLINK_NOT_FOUND


async def test_get_reconciles_status_from_chain(
    service: PayLinkService, fake_chain: Any, signer: Any
) -> None:
    row = await service.create(_cmd(signer.address))
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
    refreshed = await service.get(row.pl_id)
    assert refreshed.status == OffChainStatus.VERIFIED.value
    assert refreshed.vote_count == 3
    assert refreshed.verified_at is not None


async def test_get_tolerates_chain_outage(
    service: PayLinkService, fake_chain: Any, signer: Any
) -> None:
    row = await service.create(_cmd(signer.address))

    async def _boom(_pl_id: str) -> None:
        raise AppError(ErrorCode.CHAIN_UNAVAILABLE, "down")

    fake_chain.get_paylink = _boom  # type: ignore[assignment]
    refreshed = await service.get(row.pl_id)  # must not raise
    assert refreshed.status == OffChainStatus.PENDING.value


async def test_cancel_happy_path(service: PayLinkService, fake_chain: Any, signer: Any) -> None:
    row = await service.create(_cmd(signer.address))
    sent_before = len(fake_chain.sent)
    cancelled = await service.cancel(row.pl_id, signer.address)
    assert cancelled.status == OffChainStatus.CANCELLED.value
    assert len(fake_chain.sent) == sent_before + 1
    assert fake_chain.sent[-1]["type"] == 3  # TxCancelPayLink


async def test_cancel_requires_creator_or_owner(service: PayLinkService, signer: Any) -> None:
    row = await service.create(_cmd(signer.address))
    with pytest.raises(AppError) as exc:
        await service.cancel(row.pl_id, "0x000000000000000000000000000000000000dead")
    assert exc.value.code is ErrorCode.UNAUTHORIZED


async def test_cancel_idempotent_when_already_cancelled(
    service: PayLinkService, signer: Any
) -> None:
    row = await service.create(_cmd(signer.address))
    await service.cancel(row.pl_id, signer.address)
    again = await service.cancel(row.pl_id, signer.address)  # no error
    assert again.status == OffChainStatus.CANCELLED.value


async def test_cancel_rejects_settled(service: PayLinkService, fake_repo: Any, signer: Any) -> None:
    row = await service.create(_cmd(signer.address))
    row.status = OffChainStatus.VERIFIED.value  # simulate settled
    with pytest.raises(AppError) as exc:
        await service.cancel(row.pl_id, signer.address)
    assert exc.value.code is ErrorCode.PAYLINK_ALREADY_SETTLED


async def test_list_validates_status(service: PayLinkService) -> None:
    with pytest.raises(AppError) as exc:
        await service.list(creator=None, receiver=None, status="BOGUS", limit=10, cursor=None)
    assert exc.value.code is ErrorCode.INVALID_QUERY


async def test_cancel_emits_event(service: PayLinkService, fake_repo: Any, signer: Any) -> None:
    row = await service.create(_cmd(signer.address))
    await service.cancel(row.pl_id, signer.address)
    assert PAYLINK_CANCELLED in [k for _, k, _ in fake_repo.events]
