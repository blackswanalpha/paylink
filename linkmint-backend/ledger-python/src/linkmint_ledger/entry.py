"""Leg/Direction types and the pure balance validator (mirrors ledger-go/entry.go)."""

from __future__ import annotations

import enum
from dataclasses import dataclass

from .errors import InvalidLeg, Unbalanced


class Direction(enum.StrEnum):
    """The side of a ledger leg: DR (debit) or CR (credit)."""

    DR = "DR"
    CR = "CR"


@dataclass(frozen=True)
class Leg:
    """One half-entry of a posting: a single account movement.

    ``amount`` is in minor units (NUMERIC(38,0)) and is always strictly positive — the ``direction``
    carries the sign.
    """

    account: str
    direction: Direction
    amount: int
    currency: str


def _flip(direction: Direction) -> Direction:
    """Return the opposite direction (used by ``reverse``)."""
    return Direction.DR if direction == Direction.CR else Direction.CR


def validate(entries: list[Leg]) -> None:
    """Enforce the double-entry invariant (A.6) before any write.

    Every leg must be well-formed; the group must have at least one DR and one CR; and the DR total
    must equal the CR total for each currency. A balanced multi-currency group is allowed (each
    currency must balance on its own). Raises :class:`Unbalanced` or :class:`InvalidLeg`.
    """
    if len(entries) < 2:
        raise Unbalanced(f"a posting needs at least one DR and one CR (got {len(entries)} legs)")

    per_currency: dict[str, list[int]] = {}  # currency -> [dr_total, cr_total]
    have_dr = have_cr = False

    for i, leg in enumerate(entries):
        try:
            direction = Direction(leg.direction)
        except ValueError:
            raise InvalidLeg(f"leg {i} has invalid direction {leg.direction!r}") from None
        if not leg.account or not leg.account.strip():
            raise InvalidLeg(f"leg {i} has an empty account")
        if not leg.currency or not leg.currency.strip():
            raise InvalidLeg(f"leg {i} has an empty currency")
        # bool is a subclass of int — reject it explicitly so True/False can't be an amount.
        if isinstance(leg.amount, bool) or not isinstance(leg.amount, int) or leg.amount <= 0:
            raise InvalidLeg(f"leg {i} amount must be a positive integer")

        sums = per_currency.setdefault(leg.currency, [0, 0])
        if direction == Direction.DR:
            sums[0] += leg.amount
            have_dr = True
        else:
            sums[1] += leg.amount
            have_cr = True

    if not have_dr or not have_cr:
        raise Unbalanced("a posting needs at least one DR and one CR")
    for ccy, (dr, cr) in per_currency.items():
        if dr != cr:
            raise Unbalanced(f"currency {ccy} is unbalanced (DR={dr} CR={cr})")
