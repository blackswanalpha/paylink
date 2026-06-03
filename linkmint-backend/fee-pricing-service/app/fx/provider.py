"""FX provider port + the locked-rate value object.

A ``Rate`` is a mid-market multiplier: ``1 <base> = <rate> <quote>``. The rate is resolved once per
quote and LOCKED — stored on the quote row and in the breakdown — so a later cache refresh never
changes an issued quote (work21 invariant: rates locked at quote time, stored for audit).
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from typing import Protocol


@dataclass(frozen=True)
class Rate:
    base: str
    quote: str
    rate: Decimal
    source: str  # static | http:<host> | fallback | identity
    fetched_at: datetime


class FxProvider(Protocol):
    """Resolves a mid-market rate for ``base→quote``; returns None when it has no rate."""

    async def get_rate(self, base: str, quote: str) -> Rate | None: ...
