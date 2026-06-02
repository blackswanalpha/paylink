"""linkmint_ledger — LinkMint's shared double-entry ledger helper (work16).

The append-only ledger.ledger_entries table plus balanced posting/reversal/read helpers used
(read/write) by services to record every monetary flow for reconciliation and reporting. Postings
are balanced per currency under one entry_group (A.6); the table is append-only (a DB trigger
rejects UPDATE/DELETE), so corrections are NEW reversing entries via ``reverse()``. The helpers run
on the caller's AsyncConnection/AsyncSession and never commit, so the business write and the ledger
legs share one transaction. Non-custodial (A.1): opaque account labels, never custody. Posts into
the same shape as the Go library (ledger-go) via a byte-identical schema migration.
"""

from __future__ import annotations

from .entry import Direction, Leg, validate
from .errors import GroupNotFound, InvalidLeg, LedgerError, Unbalanced
from .ledger import (
    Entry,
    balance,
    entries_by_account,
    entries_by_group,
    entries_by_pl_id,
    is_balanced,
    post,
    reverse,
)

__all__ = [
    "Direction",
    "Leg",
    "Entry",
    "validate",
    "post",
    "reverse",
    "balance",
    "is_balanced",
    "entries_by_group",
    "entries_by_account",
    "entries_by_pl_id",
    "LedgerError",
    "Unbalanced",
    "InvalidLeg",
    "GroupNotFound",
]
