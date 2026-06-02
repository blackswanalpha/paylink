"""Pure money-aggregation + lazy-overdue logic (no I/O)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from decimal import Decimal

from app.domain.models import LineInput, compute_totals, effective_status


def test_totals_no_tax() -> None:
    lines = [
        LineInput("a", Decimal(2), 1500, Decimal(0)),
        LineInput("b", Decimal(1), 500, Decimal(0)),
    ]
    t = compute_totals(lines)
    assert t.subtotal == 3500
    assert t.tax == 0
    assert t.total == 3500
    assert [line.total for line in t.lines] == [3000, 500]


def test_totals_with_tax() -> None:
    t = compute_totals([LineInput("a", Decimal(2), 1500, Decimal("0.16"))])
    # 2 * 1500 = 3000 net; tax = round(3000 * 0.16) = 480; total = 3480
    assert t.subtotal == 3000
    assert t.tax == 480
    assert t.total == 3480


def test_totals_rounding_half_up() -> None:
    # 1.5 * 333 = 499.5 → rounds to 500
    t = compute_totals([LineInput("a", Decimal("1.5"), 333, Decimal(0))])
    assert t.lines[0].total == 500


def test_effective_status_open_past_due_is_overdue() -> None:
    now = datetime.now(UTC)
    assert effective_status("OPEN", now - timedelta(days=1), now) == "OVERDUE"
    assert effective_status("OPEN", now + timedelta(days=1), now) == "OPEN"
    assert effective_status("PAID", now - timedelta(days=1), now) == "PAID"
    assert effective_status("VOID", now - timedelta(days=1), now) == "VOID"
