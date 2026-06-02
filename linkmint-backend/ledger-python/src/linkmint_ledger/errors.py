"""Typed ledger errors (mirror the ledger-go sentinels)."""

from __future__ import annotations


class LedgerError(Exception):
    """Base class for all ledger errors."""


class Unbalanced(LedgerError):
    """A posting's DR total != CR total (per currency), or it lacks a DR/CR leg.

    Posting it would violate A.6, so it is rejected before any write.
    """


class InvalidLeg(LedgerError):
    """A malformed leg: bad direction, non-positive amount, or empty account/currency."""


class GroupNotFound(LedgerError):
    """``reverse`` referenced an entry_group with no entries."""
