"""Integration tests for the async ledger helpers against a real Postgres (testcontainers).

Mirrors ledger-go/ledger_integration_test.go: balanced post, unbalanced rejection (no rows),
DB-enforced append-only, reversal round-trip, balance/reconciliation reads, caller-transaction
composition, and a 38-digit amount round-trip.
"""

from __future__ import annotations

import uuid

import pytest
import sqlalchemy as sa
from sqlalchemy.exc import DBAPIError
from sqlalchemy.ext.asyncio import AsyncConnection, AsyncEngine, async_sessionmaker

from linkmint_ledger import (
    Direction,
    GroupNotFound,
    Leg,
    Unbalanced,
    balance,
    entries_by_account,
    entries_by_group,
    entries_by_pl_id,
    is_balanced,
    post,
    reverse,
)

pytestmark = pytest.mark.integration


async def _count(conn: AsyncConnection) -> int:
    result = await conn.execute(sa.text("SELECT count(*) FROM ledger.ledger_entries"))
    return int(result.scalar_one())


async def _post_balanced(engine: AsyncEngine) -> uuid.UUID:
    async with engine.begin() as conn:
        return await post(
            conn,
            entries=[
                Leg("paylink:PLK1", Direction.DR, 1000, "PLN"),
                Leg("treasury", Direction.CR, 1000, "PLN"),
            ],
            pl_id="PLK1",
            note="settlement",
        )


async def test_post_balanced_persists(engine: AsyncEngine) -> None:
    group = await _post_balanced(engine)
    async with engine.connect() as conn:
        entries = await entries_by_group(conn, group)
    assert len(entries) == 2
    for e in entries:
        assert e.entry_group == group
        assert e.pl_id == "PLK1"
        assert e.amount == 1000
        assert e.created_at is not None


async def test_post_unbalanced_rejected_no_rows(engine: AsyncEngine) -> None:
    with pytest.raises(Unbalanced):
        async with engine.begin() as conn:
            await post(
                conn,
                entries=[
                    Leg("a", Direction.DR, 100, "PLN"),
                    Leg("b", Direction.CR, 90, "PLN"),
                ],
            )
    async with engine.connect() as conn:
        assert await _count(conn) == 0


async def test_append_only_update_delete_rejected(engine: AsyncEngine) -> None:
    group = await _post_balanced(engine)

    with pytest.raises(DBAPIError) as update_err:
        async with engine.begin() as conn:
            await conn.exec_driver_sql(
                f"UPDATE ledger.ledger_entries SET amount = amount + 1 WHERE entry_group = '{group}'"
            )
    assert "append-only" in str(update_err.value).lower()

    with pytest.raises(DBAPIError) as delete_err:
        async with engine.begin() as conn:
            await conn.exec_driver_sql(
                f"DELETE FROM ledger.ledger_entries WHERE entry_group = '{group}'"
            )
    assert "append-only" in str(delete_err.value).lower()

    # History is intact after the rejected mutations.
    async with engine.connect() as conn:
        assert await _count(conn) == 2


async def test_reverse_round_trip(engine: AsyncEngine) -> None:
    group = await _post_balanced(engine)

    async with engine.begin() as conn:
        rev = await reverse(conn, group)
    assert rev != group

    async with engine.connect() as conn:
        for account in ("paylink:PLK1", "treasury"):
            assert await balance(conn, account, "PLN") == 0
        assert len(await entries_by_group(conn, group)) == 2  # original intact
        rev_entries = await entries_by_group(conn, rev)
    assert len(rev_entries) == 2
    by_account = {e.account: e.direction for e in rev_entries}
    assert by_account["treasury"] == Direction.DR
    assert by_account["paylink:PLK1"] == Direction.CR


async def test_reverse_missing_group_raises(engine: AsyncEngine) -> None:
    with pytest.raises(GroupNotFound):
        async with engine.begin() as conn:
            await reverse(conn, uuid.uuid4())


async def test_balance_reads_and_is_balanced(engine: AsyncEngine) -> None:
    # Fee settlement posting (0.5% fee split 70/20/10 — reuses the chain fee semantics).
    async with engine.begin() as conn:
        await post(
            conn,
            entries=[
                Leg("paylink:PLK9", Direction.DR, 1000, "PLN"),
                Leg("validator:0xabc", Direction.CR, 700, "PLN"),
                Leg("treasury", Direction.CR, 200, "PLN"),
                Leg("burn", Direction.CR, 100, "PLN"),
            ],
            pl_id="PLK9",
            note="fee 0.5% split 70/20/10",
        )

    async with engine.connect() as conn:
        assert await balance(conn, "treasury", "PLN") == 200  # ΣCR − ΣDR
        assert await balance(conn, "paylink:PLK9", "PLN") == -1000
        assert await is_balanced(conn, "PLN") is True
        assert len(await entries_by_pl_id(conn, "PLK9")) == 4
        assert len(await entries_by_account(conn, "treasury")) == 1


async def test_post_joins_caller_transaction(engine: AsyncEngine) -> None:
    sessions = async_sessionmaker(engine)
    legs = [Leg("a", Direction.DR, 5, "PLN"), Leg("b", Direction.CR, 5, "PLN")]

    # Rollback path: a post on the caller's session vanishes when the caller rolls back.
    async with sessions() as session:
        await post(session, entries=legs)
        await session.rollback()
    async with engine.connect() as conn:
        assert await _count(conn) == 0

    # Commit path: the legs persist with the caller's transaction.
    async with sessions() as session:
        await post(session, entries=legs)
        await session.commit()
    async with engine.connect() as conn:
        assert await _count(conn) == 2


async def test_big_amount_round_trip(engine: AsyncEngine) -> None:
    huge = 12345678901234567890123456789012345678  # 38 digits, > 2**63
    async with engine.begin() as conn:
        group = await post(
            conn,
            entries=[
                Leg("a", Direction.DR, huge, "PLN"),
                Leg("b", Direction.CR, huge, "PLN"),
            ],
        )
    async with engine.connect() as conn:
        entries = await entries_by_group(conn, group)
    assert all(e.amount == huge for e in entries)
