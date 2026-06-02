"""Pure domain model: the invoice lifecycle enum + deterministic money aggregation.

Money is integer **minor units** (e.g. cents). ``unit_price`` and all invoice/line totals are
integers; ``quantity`` (4 dp) and ``tax_rate`` (4 dp) are decimals. Line total = round(quantity ×
unit_price); line tax = round(line_total × tax_rate); invoice subtotal/tax/total are the sums. All
rounding is HALF_UP at 0 dp. This module has NO I/O — it's unit-tested directly.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import ROUND_HALF_UP, Decimal
from enum import StrEnum


class InvoiceStatus(StrEnum):
    DRAFT = "DRAFT"
    OPEN = "OPEN"
    PAID = "PAID"
    VOID = "VOID"
    OVERDUE = "OVERDUE"


@dataclass(frozen=True)
class LineInput:
    """A validated invoice line as the service consumes it (minor-unit unit_price)."""

    description: str
    quantity: Decimal
    unit_price: int
    tax_rate: Decimal = Decimal(0)


@dataclass(frozen=True)
class ComputedLine:
    description: str
    quantity: Decimal
    unit_price: int
    total: int  # net line amount (pre-tax), minor units
    tax_rate: Decimal


@dataclass(frozen=True)
class ComputedTotals:
    lines: list[ComputedLine]
    subtotal: int
    tax: int
    total: int


def _round0(value: Decimal) -> int:
    return int(value.quantize(Decimal(1), rounding=ROUND_HALF_UP))


def compute_totals(lines: list[LineInput]) -> ComputedTotals:
    """Aggregate lines → (per-line totals, subtotal, tax, grand total). All integer minor units."""
    computed: list[ComputedLine] = []
    subtotal = 0
    tax = 0
    for ln in lines:
        line_total = _round0(ln.quantity * Decimal(ln.unit_price))
        line_tax = _round0(Decimal(line_total) * ln.tax_rate)
        subtotal += line_total
        tax += line_tax
        computed.append(
            ComputedLine(
                description=ln.description,
                quantity=ln.quantity,
                unit_price=ln.unit_price,
                total=line_total,
                tax_rate=ln.tax_rate,
            )
        )
    return ComputedTotals(lines=computed, subtotal=subtotal, tax=tax, total=subtotal + tax)


def effective_status(status: str, due_at: datetime, now: datetime) -> str:
    """Lazy OVERDUE reflection: an OPEN invoice past its due date reads as OVERDUE (the sweeper is
    what eventually persists the transition + emits ``invoice.overdue``)."""
    if status == InvoiceStatus.OPEN.value and due_at < now:
        return InvoiceStatus.OVERDUE.value
    return status
