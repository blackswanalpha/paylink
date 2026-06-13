"""Rail-reversal seam (rules.md A.1 — non-custodial, instruction-only).

work22's "rail-specific reversal" (MPesa B2C, Stripe refund, crypto outbound, bank ACH return) has
no implementation yet: the MPesa adapter is STK-push-only ("never B2C/reversal/sweep", see
``adapters/mpesa/DESIGN.md``) and the card/crypto/bank adapters are work28-30. So the reversal is a
PORT with an instruction-only default: the service writes a ``refund.reversal.instructed`` event
(the
INSTRUCTION) and the port returns an opaque ``reversal_ref``. A real rail adapter slots into the
:class:`RailReversalRegistry` later with NO call-site change — exactly the seam pattern work21 used
for the ledger and work11 used for the audit sink.
"""

from __future__ import annotations

from typing import Protocol


class RailReversal(Protocol):
    async def instruct(
        self,
        *,
        refund_id: str,
        rail: str,
        amount_minor: int,
        currency: str,
        paylink_id: str,
        payment_id: str,
    ) -> str | None:
        """Instruct the rail to reverse ``amount_minor``. Returns an opaque ``reversal_ref`` (or
        None
        when the rail has no synchronous handle). MUST NOT move funds itself (A.1)."""
        ...


class RailReversalRegistry:
    """Selects the reversal strategy for a payment's rail. Today every rail maps to the
    instruction-only default; work28-30 register real adapters here."""

    def __init__(
        self, default: RailReversal, overrides: dict[str, RailReversal] | None = None
    ) -> None:
        self._default = default
        self._by_rail = dict(overrides or {})

    def for_rail(self, rail: str) -> RailReversal:
        return self._by_rail.get(rail, self._default)
