"""Static FX provider — deterministic rates from a config string.

Format: ``"USD:KES=129.50,EUR:KES=140.00,KES:KES=1"`` (``base:quote=rate``, comma-separated). Used
as the default provider (dev/test) and as the fallback table when a live provider returns nothing.
"""

from __future__ import annotations

from datetime import UTC, datetime
from decimal import Decimal, InvalidOperation

from app.fx.provider import Rate
from app.logging import get_logger

log = get_logger("pricing.fx")


def parse_rate_table(spec: str) -> dict[tuple[str, str], Decimal]:
    """Parse ``base:quote=rate,...`` into ``{(BASE, QUOTE): Decimal}``. Bad entries are skipped."""
    table: dict[tuple[str, str], Decimal] = {}
    for entry in spec.split(","):
        entry = entry.strip()
        if not entry:
            continue
        try:
            pair, rate = entry.split("=", 1)
            base, quote = pair.split(":", 1)
            table[(base.strip().upper(), quote.strip().upper())] = Decimal(rate.strip())
        except (ValueError, InvalidOperation):
            log.warning("fx_rate_spec_skipped", entry=entry)
    return table


class StaticFxProvider:
    """Serves rates from a parsed table. Identity pairs (BASE==QUOTE) always resolve to 1."""

    def __init__(self, spec: str, *, source: str = "static") -> None:
        self._table = parse_rate_table(spec)
        self._source = source

    async def get_rate(self, base: str, quote: str) -> Rate | None:
        base, quote = base.upper(), quote.upper()
        if base == quote:
            return Rate(base, quote, Decimal(1), "identity", datetime.now(UTC))
        rate = self._table.get((base, quote))
        if rate is None:
            return None
        return Rate(base, quote, rate, self._source, datetime.now(UTC))
