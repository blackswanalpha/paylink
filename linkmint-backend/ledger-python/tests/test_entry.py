"""Pure-unit tests for the balance validator (no DB). Mirrors ledger-go/entry_test.go."""

from __future__ import annotations

import pytest

from linkmint_ledger import Direction, Leg, validate
from linkmint_ledger.errors import InvalidLeg, Unbalanced


def leg(
    account: str = "a",
    direction: Direction = Direction.DR,
    amount: int = 100,
    currency: str = "PLN",
) -> Leg:
    return Leg(account=account, direction=direction, amount=amount, currency=currency)


def test_balanced_single_currency() -> None:
    validate(
        [leg("paylink:PLK1", Direction.DR, 100, "PLN"), leg("treasury", Direction.CR, 100, "PLN")]
    )


def test_balanced_fee_split_70_20_10() -> None:
    validate(
        [
            leg("paylink:PLK1", Direction.DR, 1000, "PLN"),
            leg("validator:0xabc", Direction.CR, 700, "PLN"),
            leg("treasury", Direction.CR, 200, "PLN"),
            leg("burn", Direction.CR, 100, "PLN"),
        ]
    )


def test_multi_currency_balanced_per_currency() -> None:
    validate(
        [
            leg("a", Direction.DR, 100, "PLN"),
            leg("b", Direction.CR, 100, "PLN"),
            leg("c", Direction.DR, 50, "USD"),
            leg("d", Direction.CR, 50, "USD"),
        ]
    )


def test_direction_accepts_str() -> None:
    # A leg built with the raw string "DR"/"CR" validates the same as the enum.
    validate([Leg("a", "DR", 5, "PLN"), Leg("b", "CR", 5, "PLN")])  # type: ignore[arg-type]


@pytest.mark.parametrize(
    "entries",
    [
        pytest.param(
            [leg("a", Direction.DR, 100, "PLN"), leg("b", Direction.CR, 90, "PLN")],
            id="amounts-differ",
        ),
        pytest.param([], id="empty"),
        pytest.param([leg("a", Direction.DR, 100, "PLN")], id="single-leg"),
        pytest.param(
            [leg("a", Direction.DR, 50, "PLN"), leg("b", Direction.DR, 50, "PLN")], id="all-dr"
        ),
        pytest.param(
            [
                leg("a", Direction.DR, 100, "PLN"),
                leg("b", Direction.CR, 100, "PLN"),
                leg("c", Direction.DR, 50, "USD"),
                leg("d", Direction.CR, 40, "USD"),
            ],
            id="one-currency-unbalanced",
        ),
    ],
)
def test_unbalanced_rejected(entries: list[Leg]) -> None:
    with pytest.raises(Unbalanced):
        validate(entries)


@pytest.mark.parametrize(
    "entries",
    [
        pytest.param([Leg("a", "XX", 100, "PLN"), leg("b", Direction.CR, 100, "PLN")], id="bad-direction"),  # type: ignore[arg-type]
        pytest.param(
            [leg("a", Direction.DR, 0, "PLN"), leg("b", Direction.CR, 0, "PLN")], id="zero-amount"
        ),
        pytest.param(
            [leg("a", Direction.DR, -5, "PLN"), leg("b", Direction.CR, -5, "PLN")],
            id="negative-amount",
        ),
        pytest.param(
            [leg("  ", Direction.DR, 100, "PLN"), leg("b", Direction.CR, 100, "PLN")],
            id="empty-account",
        ),
        pytest.param(
            [leg("a", Direction.DR, 100, ""), leg("b", Direction.CR, 100, "")], id="empty-currency"
        ),
        pytest.param([Leg("a", Direction.DR, True, "PLN"), leg("b", Direction.CR, 1, "PLN")], id="bool-amount"),  # type: ignore[arg-type]
    ],
)
def test_invalid_leg_rejected(entries: list[Leg]) -> None:
    with pytest.raises(InvalidLeg):
        validate(entries)


def test_huge_amount_is_valid() -> None:
    # Python ints are unbounded — a 38-digit minor-unit amount (NUMERIC(38,0) ceiling) is fine.
    huge = 99999999999999999999999999999999999999
    validate([Leg("a", Direction.DR, huge, "PLN"), Leg("b", Direction.CR, huge, "PLN")])
