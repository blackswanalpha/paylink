"""Chain-event projection consumer (chain.paylink.verified → verified_paylinks)."""

from __future__ import annotations

from app.events.consumer import ChainEventConsumer
from tests._support import FakeRefundRepository


class _Sink:
    def __init__(self, repo: FakeRefundRepository) -> None:
        self._repo = repo

    async def project_verified_paylink(self, **kwargs: object) -> None:
        await self._repo.upsert_verified_paylink(**kwargs)  # type: ignore[arg-type]


async def test_projects_verified_paylink() -> None:
    repo = FakeRefundRepository()
    consumer = ChainEventConsumer(_Sink(repo))
    await consumer.handle(
        "chain.paylink.verified",
        {
            "entity_id": "0xpl",
            "tx_hash": "0xabc",
            "block_height": 42,
            "timestamp": 1700000000,
            "data": {"amount": 1000, "currency": "KES"},
        },
    )
    vp = await repo.get_verified_paylink("0xpl")
    assert vp is not None
    assert int(vp.amount_minor) == 1000
    assert vp.tx_hash == "0xabc"
    assert vp.block_height == 42


async def test_ignores_other_events() -> None:
    repo = FakeRefundRepository()
    consumer = ChainEventConsumer(_Sink(repo))
    await consumer.handle("chain.paylink.cancelled", {"entity_id": "0xpl"})
    assert await repo.get_verified_paylink("0xpl") is None


async def test_missing_plid_noop() -> None:
    repo = FakeRefundRepository()
    consumer = ChainEventConsumer(_Sink(repo))
    await consumer.handle("chain.paylink.verified", {"data": {"amount": 5}})
    assert repo.verified == {}


async def test_missing_data_tolerated() -> None:
    repo = FakeRefundRepository()
    consumer = ChainEventConsumer(_Sink(repo))
    await consumer.handle("chain.paylink.verified", {"entity_id": "0xpl2"})
    vp = await repo.get_verified_paylink("0xpl2")
    assert vp is not None
    assert vp.amount_minor is None
