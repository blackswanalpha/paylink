"""Async posting/reversal/read helpers over a caller's SQLAlchemy connection.

Mirrors ledger-go/ledger.go. The helpers run on the caller's ``AsyncConnection`` or
``AsyncSession`` and NEVER commit — the caller's unit-of-work owns the transaction boundary, so a
business-state write and the ledger legs commit together (A.6) or roll back together. Non-custodial
(A.1): this records flows between opaque account labels (e.g. ``paylink:PLK...``, ``treasury``,
``validator:0x...``); it never holds or moves funds. Amounts are exact integers (minor units);
balances are read-only sums.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncConnection, AsyncSession

from .entry import Direction, Leg, _flip, validate
from .errors import GroupNotFound

# A caller may pass either an AsyncConnection or an AsyncSession; both share one transaction.
Conn = AsyncConnection | AsyncSession

_SELECT = (
    "SELECT id, entry_group::text AS entry_group, account, direction, amount::text AS amount, "
    "currency, pl_id, note, created_at FROM ledger.ledger_entries"
)


@dataclass(frozen=True)
class Entry:
    """A persisted ledger row (one leg) as read back from the table."""

    id: int
    entry_group: uuid.UUID
    account: str
    direction: Direction
    amount: int
    currency: str
    pl_id: str | None
    note: str | None
    created_at: datetime


async def _as_conn(db: Conn) -> AsyncConnection:
    """Resolve an AsyncSession to its underlying AsyncConnection; pass a connection through."""
    if isinstance(db, AsyncSession):
        return await db.connection()
    return db


def _clamp(limit: int) -> int:
    if limit <= 0:
        return 100
    if limit > 1000:
        return 1000
    return limit


def _row_to_entry(row: object) -> Entry:
    return Entry(
        id=row.id,  # type: ignore[attr-defined]
        entry_group=uuid.UUID(row.entry_group),  # type: ignore[attr-defined]
        account=row.account,  # type: ignore[attr-defined]
        direction=Direction(row.direction),  # type: ignore[attr-defined]
        amount=int(row.amount),  # type: ignore[attr-defined]
        currency=row.currency,  # type: ignore[attr-defined]
        pl_id=row.pl_id,  # type: ignore[attr-defined]
        note=row.note,  # type: ignore[attr-defined]
        created_at=row.created_at,  # type: ignore[attr-defined]
    )


async def post(
    db: Conn,
    *,
    entries: list[Leg],
    entry_group: uuid.UUID | None = None,
    pl_id: str | None = None,
    note: str | None = None,
) -> uuid.UUID:
    """Validate a balanced set of legs and write them atomically under one entry_group.

    The legs are inserted in a single statement (atomic), so a group is all-or-nothing; when ``db``
    is a caller's session/connection inside a transaction, the legs join it. Returns the
    entry_group. Rejects unbalanced/malformed input (:class:`Unbalanced` / :class:`InvalidLeg`)
    before any write.
    """
    validate(entries)
    group = entry_group or uuid.uuid4()
    conn = await _as_conn(db)

    rows: list[str] = []
    params: dict[str, object] = {"g": str(group), "pl": pl_id, "note": note}
    for i, leg in enumerate(entries):
        # CAST(... AS ...) rather than ::uuid / ::numeric — SQLAlchemy text() will not bind a
        # ``:name`` that is immediately followed by ``::``. Amounts bind as strings (exact NUMERIC).
        rows.append(
            f"(CAST(:g AS uuid), :acct{i}, :dir{i}, "
            f"CAST(:amt{i} AS numeric), :ccy{i}, :pl, :note)"
        )
        params[f"acct{i}"] = leg.account
        params[f"dir{i}"] = Direction(leg.direction).value
        params[f"amt{i}"] = str(leg.amount)
        params[f"ccy{i}"] = leg.currency

    sql = (
        "INSERT INTO ledger.ledger_entries "
        "(entry_group, account, direction, amount, currency, pl_id, note) VALUES " + ", ".join(rows)
    )
    await conn.execute(text(sql), params)
    return group


async def reverse(db: Conn, entry_group: uuid.UUID, *, note: str | None = None) -> uuid.UUID:
    """Post a correcting entry group: read the original group's legs and write a NEW group with
    every direction flipped (DR<->CR), same amounts/currency/account/pl_id. The original is never
    edited or deleted (A.6). Returns the new entry_group. Raises :class:`GroupNotFound` if empty.
    """
    entries = await entries_by_group(db, entry_group)
    if not entries:
        raise GroupNotFound(str(entry_group))
    pl_id = next((e.pl_id for e in entries if e.pl_id), None)
    flipped = [
        Leg(account=e.account, direction=_flip(e.direction), amount=e.amount, currency=e.currency)
        for e in entries
    ]
    return await post(db, entries=flipped, pl_id=pl_id, note=note or f"reversal of {entry_group}")


async def entries_by_group(db: Conn, entry_group: uuid.UUID) -> list[Entry]:
    """Return all legs of an entry_group, oldest id first."""
    conn = await _as_conn(db)
    result = await conn.execute(
        text(_SELECT + " WHERE entry_group = CAST(:g AS uuid) ORDER BY id"),
        {"g": str(entry_group)},
    )
    return [_row_to_entry(r) for r in result.all()]


async def entries_by_account(db: Conn, account: str, limit: int = 100) -> list[Entry]:
    """Return the most recent legs for an account (newest first), capped by ``limit`` (max 1000)."""
    conn = await _as_conn(db)
    result = await conn.execute(
        text(_SELECT + " WHERE account=:a ORDER BY created_at DESC, id DESC LIMIT :n"),
        {"a": account, "n": _clamp(limit)},
    )
    return [_row_to_entry(r) for r in result.all()]


async def entries_by_pl_id(db: Conn, pl_id: str, limit: int = 100) -> list[Entry]:
    """Most recent legs tied to a pl_id (newest first), capped by ``limit`` (max 1000)."""
    conn = await _as_conn(db)
    result = await conn.execute(
        text(_SELECT + " WHERE pl_id=:p ORDER BY created_at DESC, id DESC LIMIT :n"),
        {"p": pl_id, "n": _clamp(limit)},
    )
    return [_row_to_entry(r) for r in result.all()]


async def balance(db: Conn, account: str, currency: str) -> int:
    """Net balance of an account in a currency as ΣCR − ΣDR (credit-positive). Read-only (A.1)."""
    conn = await _as_conn(db)
    result = await conn.execute(
        text(
            "SELECT COALESCE(SUM(CASE WHEN direction='CR' THEN amount ELSE -amount END), 0)::text "
            "FROM ledger.ledger_entries WHERE account=:a AND currency=:c"
        ),
        {"a": account, "c": currency},
    )
    return int(result.scalar_one())


async def is_balanced(db: Conn, currency: str) -> bool:
    """Whether the whole ledger nets to zero for a currency (ΣDR == ΣCR). work27 reconciliation."""
    conn = await _as_conn(db)
    result = await conn.execute(
        text(
            "SELECT COALESCE(SUM(CASE WHEN direction='DR' THEN amount ELSE -amount END), 0)::text "
            "FROM ledger.ledger_entries WHERE currency=:c"
        ),
        {"c": currency},
    )
    return int(result.scalar_one()) == 0
