"""Clawback coordination seam (work23 settlement).

On a lost/expired dispute the platform must claw back the disputed amount from the merchant's NEXT
payout. The settlement service (work23) owns payout mechanics and is not built yet, so the
coordination is a published-event CONTRACT, not a synchronous call:
:class:`EventClawbackCoordinator`
writes a ``refund.clawback.requested`` outbox row that settlement will consume when it lands. This
is
the same "interface + default impl + config gate" seam work11 used for the audit sink before work13.

Non-custodial (A.1): this service never moves funds — it only requests the clawback.
"""

from __future__ import annotations

from typing import Protocol

from app.db.repositories import RefundRepository
from app.events.publisher import REFUND_CLAWBACK_REQUESTED
from app.logging import get_logger

log = get_logger("refund.clawback")


class ClawbackCoordinator(Protocol):
    async def request_clawback(
        self,
        repo: RefundRepository,
        *,
        dispute_id: str,
        payment_id: str | None,
        paylink_id: str | None,
        merchant_id: str | None,
        org_id: str | None,
        amount_minor: int | None,
        currency: str | None,
        reason: str,
    ) -> None: ...


class EventClawbackCoordinator:
    """Default — writes the ``refund.clawback.requested`` outbox row on the caller's transaction."""

    async def request_clawback(
        self,
        repo: RefundRepository,
        *,
        dispute_id: str,
        payment_id: str | None,
        paylink_id: str | None,
        merchant_id: str | None,
        org_id: str | None,
        amount_minor: int | None,
        currency: str | None,
        reason: str,
    ) -> None:
        payload = {
            "dispute_id": dispute_id,
            "payment_id": payment_id,
            # paylink_id lets settlement-service (work23) resolve the merchant + currency from the
            # PayLink it already settled, so the clawback offsets the right merchant's next payout.
            "paylink_id": paylink_id,
            "merchant_id": merchant_id,
            "org_id": org_id,
            "amount_minor": amount_minor,
            "currency": currency,
            "reason": reason,
        }
        await repo.add_event(dispute_id, REFUND_CLAWBACK_REQUESTED, payload)
        log.info("clawback_requested", dispute_id=dispute_id, reason=reason)


class NoopClawbackCoordinator:
    """Records nothing (used in tests / when clawback is disabled)."""

    async def request_clawback(
        self,
        repo: RefundRepository,
        *,
        dispute_id: str,
        payment_id: str | None,
        paylink_id: str | None,
        merchant_id: str | None,
        org_id: str | None,
        amount_minor: int | None,
        currency: str | None,
        reason: str,
    ) -> None:
        log.info("clawback_skipped", dispute_id=dispute_id, reason=reason)


def build_clawback_coordinator(mode: str) -> ClawbackCoordinator:
    return NoopClawbackCoordinator() if mode == "noop" else EventClawbackCoordinator()
