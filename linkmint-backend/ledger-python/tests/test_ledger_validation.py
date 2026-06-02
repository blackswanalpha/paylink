"""Pure-unit tests for the small ledger helpers (no DB)."""

from __future__ import annotations

from linkmint_ledger.entry import Direction, _flip
from linkmint_ledger.ledger import _clamp


def test_flip() -> None:
    assert _flip(Direction.DR) == Direction.CR
    assert _flip(Direction.CR) == Direction.DR


def test_clamp_limit() -> None:
    assert _clamp(0) == 100
    assert _clamp(-5) == 100
    assert _clamp(1) == 1
    assert _clamp(50) == 50
    assert _clamp(1000) == 1000
    assert _clamp(5000) == 1000
