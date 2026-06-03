"""Pure pricing domain: tier/rail enums + deterministic quote math. NO I/O — unit-tested directly.

Money is integer **minor units** (e.g. cents). Percentages are integer **basis points** (250 =
2.50%). All rounding is HALF_UP at 0 dp. The FX rate is resolved + locked by the FX service and
passed IN, so this module stays pure and the rate used is exactly the rate persisted on the quote.

The platform fee here is the LinkMint platform's pricing fee — DISTINCT from the on-chain 0.5% PLN
inflation fee (rules.md A.5). This module never touches chain-fee logic.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import ROUND_HALF_UP, Decimal
from enum import StrEnum
from typing import Any

from app.errors import AppError, ErrorCode
from app.fx.provider import Rate

BPS_DENOM = 10_000


class Tier(StrEnum):
    """Superset of work21 (standard/growth/scale/enterprise) + work10 (adds startup)."""

    STANDARD = "standard"
    STARTUP = "startup"
    GROWTH = "growth"
    SCALE = "scale"
    ENTERPRISE = "enterprise"


class Rail(StrEnum):
    """Settlement rails — the rail-agnostic protocol set (rules.md A.4)."""

    MPESA = "mpesa"
    CARD = "card"
    BANK = "bank"
    CRYPTO = "crypto"


# The tier a merchant resolves to when none is known/recognized (work10 emits only a subset).
DEFAULT_TIER = Tier.STANDARD.value


@dataclass(frozen=True)
class TierFee:
    """A tier's platform-fee schedule (pure value object built from a TierRow)."""

    tier: str
    platform_pct_bps: int
    platform_fixed: int  # minor units, in `fixed_currency`
    fixed_currency: str


@dataclass(frozen=True)
class RailFee:
    """A rail's fee schedule (pure value object built from a RailFeeRow)."""

    rail: str
    pct_bps: int
    fixed: int  # minor units, in `fixed_currency`
    fixed_currency: str


@dataclass(frozen=True)
class QuoteResult:
    gross: int
    currency: str
    gross_settled: int
    settle_currency: str
    platform_pct_amount: int
    platform_fixed_amount: int
    platform_fee: int
    rail_pct_amount: int
    rail_fixed_amount: int
    rail_fee: int
    net: int
    fx: Rate | None
    breakdown: dict[str, Any]


def _round0(value: Decimal) -> int:
    return int(value.quantize(Decimal(1), rounding=ROUND_HALF_UP))


def _convert(amount: int, from_ccy: str, settle_ccy: str, fx: Rate | None) -> int:
    """Convert a fixed component into the settlement currency using the same locked rate.

    Same-currency is a no-op. Cross-currency needs ``fx`` to be exactly the ``from_ccy→settle_ccy``
    rate (the gross-conversion rate already covers the common case where the fixed fee is in the
    gross currency). A fixed fee in some unrelated third currency is unsupported → FX_UNAVAILABLE.
    """
    if amount == 0 or from_ccy == settle_ccy:
        return amount
    if fx is not None and fx.base == from_ccy and fx.quote == settle_ccy:
        return _round0(Decimal(amount) * fx.rate)
    raise AppError(
        ErrorCode.FX_UNAVAILABLE,
        f"no locked rate to convert fixed fee {from_ccy}->{settle_ccy}",
        details={"from": from_ccy, "to": settle_ccy},
    )


def compute_quote(
    *,
    gross_minor: int,
    currency: str,
    settle_currency: str,
    tier: TierFee,
    rail: RailFee,
    fx: Rate | None,
    locked_at: str,
) -> QuoteResult:
    """Compute a full pricing quote for one (tier, rail) pair, in integer minor units.

    ``fx`` must be the locked ``currency→settle_currency`` rate when the two differ (else None).
    Raises ``FEE_EXCEEDS_GROSS`` when the fees exceed the payment (negative net), and
    ``FX_UNAVAILABLE`` when a required conversion has no rate.
    """
    currency = currency.upper()
    settle_currency = settle_currency.upper()

    # 1. Convert gross into the settlement currency (rate locked upstream).
    if currency == settle_currency:
        gross_settled = gross_minor
        fx_used: Rate | None = None
    else:
        if fx is None or fx.base != currency or fx.quote != settle_currency:
            raise AppError(
                ErrorCode.FX_UNAVAILABLE,
                f"no locked rate for {currency}->{settle_currency}",
                details={"from": currency, "to": settle_currency},
            )
        gross_settled = _round0(Decimal(gross_minor) * fx.rate)
        fx_used = fx

    # 2. Platform fee = pct of gross_settled + flat (flat converted with the same locked rate).
    platform_pct = _round0(Decimal(gross_settled) * tier.platform_pct_bps / BPS_DENOM)
    platform_fixed = _convert(
        tier.platform_fixed, tier.fixed_currency.upper(), settle_currency, fx_used
    )
    platform_fee = platform_pct + platform_fixed

    # 3. Rail fee = pct of gross_settled + flat (same flat-conversion rule).
    rail_pct = _round0(Decimal(gross_settled) * rail.pct_bps / BPS_DENOM)
    rail_fixed = _convert(rail.fixed, rail.fixed_currency.upper(), settle_currency, fx_used)
    rail_fee = rail_pct + rail_fixed

    # 4. Net (informational — non-custodial: LinkMint never holds the funds, A.1).
    net = gross_settled - platform_fee - rail_fee
    if net < 0:
        raise AppError(
            ErrorCode.FEE_EXCEEDS_GROSS,
            "fees exceed the payment amount",
            details={
                "gross_settled": gross_settled,
                "platform_fee": platform_fee,
                "rail_fee": rail_fee,
            },
        )

    breakdown: dict[str, Any] = {
        "gross": {"amount": gross_minor, "currency": currency},
        "gross_settled": {"amount": gross_settled, "currency": settle_currency},
        "platform_fee": {
            "pct_bps": tier.platform_pct_bps,
            "pct_amount": platform_pct,
            "fixed_amount": platform_fixed,
            "amount": platform_fee,
            "currency": settle_currency,
        },
        "rail_fee": {
            "rail": rail.rail,
            "pct_bps": rail.pct_bps,
            "pct_amount": rail_pct,
            "fixed_amount": rail_fixed,
            "amount": rail_fee,
            "currency": settle_currency,
        },
        "net": {"amount": net, "currency": settle_currency},
        "fx": (
            None
            if fx_used is None
            else {
                "base": fx_used.base,
                "quote": fx_used.quote,
                "rate": str(fx_used.rate),
                "locked_at": locked_at,
                "source": fx_used.source,
            }
        ),
    }
    return QuoteResult(
        gross=gross_minor,
        currency=currency,
        gross_settled=gross_settled,
        settle_currency=settle_currency,
        platform_pct_amount=platform_pct,
        platform_fixed_amount=platform_fixed,
        platform_fee=platform_fee,
        rail_pct_amount=rail_pct,
        rail_fixed_amount=rail_fixed,
        rail_fee=rail_fee,
        net=net,
        fx=fx_used,
        breakdown=breakdown,
    )
