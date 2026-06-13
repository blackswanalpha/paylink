"""The instruction-only rail-reversal default (the MVP for every rail).

It records the intent and returns a deterministic ``reversal_ref`` derived from the refund id; the
authoritative instruction to the rail is the ``refund.reversal.instructed`` event the service emits.
Swap in a real per-rail adapter via :class:`RailReversalRegistry` when work28-30 land — no call-site
change is needed.
"""

from __future__ import annotations

from app.logging import get_logger

log = get_logger("refund.reversal")


class InstructionOnlyReversal:
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
        log.info(
            "reversal_instructed",
            refund_id=refund_id,
            rail=rail,
            amount_minor=amount_minor,
            currency=currency,
            paylink_id=paylink_id,
            payment_id=payment_id,
            mode="instruction_only",
        )
        return f"instruction:{rail}:{refund_id}"
