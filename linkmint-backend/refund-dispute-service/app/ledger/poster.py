"""Double-entry ledger seam (rules.md A.6) — OFF by default.

A completed refund and a clawback are monetary flows, so A.6 says they should eventually have
matching debit/credit entries. But work16 explicitly defers per-service ledger posting (the
settlement hub, work23, is the intended writer). So we ship a port + a no-op default and gate the
call on ``REFUND_LEDGER_POSTING_ENABLED`` (false): the seam honors A.6 intent and makes the future
switch a one-line config + a real ledger-python adapter, with zero coupling now and no risk of
double-posting. Mirrors fee-pricing-service's ledger seam.
"""

from __future__ import annotations

from typing import Protocol

from app.logging import get_logger

log = get_logger("refund.ledger")


class LedgerPoster(Protocol):
    async def post_refund(
        self,
        *,
        refund_id: str,
        payment_id: str,
        amount_minor: int,
        currency: str,
    ) -> None: ...


class NoopLedgerPoster:
    """The default poster — records nothing, logs the intent. Swap for a ledger-python adapter when
    work16 per-service posting / work23 settlement posting is enabled."""

    async def post_refund(
        self,
        *,
        refund_id: str,
        payment_id: str,
        amount_minor: int,
        currency: str,
    ) -> None:
        log.info(
            "ledger_post_skipped",
            refund_id=refund_id,
            payment_id=payment_id,
            amount_minor=amount_minor,
            currency=currency,
        )
