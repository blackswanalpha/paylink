"""Golden vectors for the pure quote math (no I/O). Integer minor units, HALF_UP, no floats."""

from __future__ import annotations

from datetime import UTC, datetime
from decimal import Decimal

import pytest

from app.domain.models import RailFee, TierFee, compute_quote
from app.errors import AppError, ErrorCode
from app.fx.provider import Rate

LOCKED = "2026-06-03T00:00:00+00:00"


def _tier(pct_bps: int, fixed: int = 0, ccy: str = "KES") -> TierFee:
    return TierFee(
        tier="standard", platform_pct_bps=pct_bps, platform_fixed=fixed, fixed_currency=ccy
    )


def _rail(pct_bps: int, fixed: int = 0, ccy: str = "KES") -> RailFee:
    return RailFee(rail="mpesa", pct_bps=pct_bps, fixed=fixed, fixed_currency=ccy)


def test_same_currency_no_fx() -> None:
    q = compute_quote(
        gross_minor=100_000,
        currency="KES",
        settle_currency="KES",
        tier=_tier(250),  # 2.50%
        rail=_rail(150),  # 1.50%
        fx=None,
        locked_at=LOCKED,
    )
    assert q.gross_settled == 100_000
    assert q.platform_fee == 2_500
    assert q.rail_fee == 1_500
    assert q.net == 96_000
    assert q.fx is None
    assert q.breakdown["fx"] is None
    assert set(q.breakdown.keys()) == {
        "gross",
        "gross_settled",
        "platform_fee",
        "rail_fee",
        "net",
        "fx",
    }


@pytest.mark.parametrize(
    "gross,bps,expected",
    [
        (333, 250, 8),  # 8.325 → 8 (round down)
        (340, 250, 9),  # 8.5   → 9 (HALF_UP rounds .5 up)
        (350, 250, 9),  # 8.75  → 9
    ],
)
def test_half_up_rounding_boundaries(gross: int, bps: int, expected: int) -> None:
    q = compute_quote(
        gross_minor=gross,
        currency="KES",
        settle_currency="KES",
        tier=_tier(bps),
        rail=_rail(0),
        fx=None,
        locked_at=LOCKED,
    )
    assert q.platform_fee == expected


def test_cross_currency_locks_rate() -> None:
    fx = Rate("USD", "KES", Decimal("129.50"), "static", datetime.now(UTC))
    q = compute_quote(
        gross_minor=100,  # 1.00 USD
        currency="USD",
        settle_currency="KES",
        tier=_tier(250),
        rail=_rail(150),
        fx=fx,
        locked_at=LOCKED,
    )
    assert q.gross_settled == 12_950  # round0(100 * 129.50)
    assert q.platform_fee == 324  # round0(12950 * 0.025) = 323.75 → 324
    assert q.rail_fee == 194  # round0(12950 * 0.015) = 194.25 → 194
    assert q.net == 12_432
    assert q.fx is not None and q.breakdown["fx"]["rate"] == "129.50"
    assert q.breakdown["fx"]["locked_at"] == LOCKED


def test_fixed_component_converted_with_locked_rate() -> None:
    fx = Rate("USD", "KES", Decimal("129.50"), "static", datetime.now(UTC))
    q = compute_quote(
        gross_minor=100,
        currency="USD",
        settle_currency="KES",
        tier=_tier(0, fixed=50, ccy="USD"),  # 0.50 USD flat → converted to KES
        rail=_rail(0),
        fx=fx,
        locked_at=LOCKED,
    )
    assert q.platform_fixed_amount == 6_475  # round0(50 * 129.50)
    assert q.platform_fee == 6_475


def test_same_currency_fixed_passes_through() -> None:
    q = compute_quote(
        gross_minor=100_000,
        currency="KES",
        settle_currency="KES",
        tier=_tier(0),
        rail=_rail(290, fixed=30),  # card-like: 2.90% + 30 flat
        fx=None,
        locked_at=LOCKED,
    )
    assert q.rail_fee == 2_900 + 30


def test_net_negative_raises() -> None:
    with pytest.raises(AppError) as exc:
        compute_quote(
            gross_minor=100,
            currency="KES",
            settle_currency="KES",
            tier=_tier(5000),  # 50%
            rail=_rail(6000),  # 60% → fees 110 > 100
            fx=None,
            locked_at=LOCKED,
        )
    assert exc.value.code == ErrorCode.FEE_EXCEEDS_GROSS


def test_cross_currency_without_rate_raises() -> None:
    with pytest.raises(AppError) as exc:
        compute_quote(
            gross_minor=100,
            currency="USD",
            settle_currency="KES",
            tier=_tier(250),
            rail=_rail(150),
            fx=None,  # missing required rate
            locked_at=LOCKED,
        )
    assert exc.value.code == ErrorCode.FX_UNAVAILABLE


def test_fixed_in_unrelated_third_currency_raises() -> None:
    fx = Rate("USD", "KES", Decimal("129.50"), "static", datetime.now(UTC))
    with pytest.raises(AppError) as exc:
        compute_quote(
            gross_minor=100,
            currency="USD",
            settle_currency="KES",
            tier=_tier(0, fixed=10, ccy="EUR"),  # EUR fixed, no EUR→KES rate available
            rail=_rail(0),
            fx=fx,
            locked_at=LOCKED,
        )
    assert exc.value.code == ErrorCode.FX_UNAVAILABLE
