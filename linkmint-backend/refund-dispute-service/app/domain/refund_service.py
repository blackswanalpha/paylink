"""Refund domain service: the REQUESTED→PROCESSING→COMPLETED lifecycle (full + partial).

The HTTP path (deps.get_services) and the sweeper build the same Services bundle over a fresh
session, so the rules live in one place. Eligibility is gated on the payment being SETTLED
(payment-orchestrator); the original amount (for full/partial + the cumulative cap) is resolved from
the verified_paylinks projection with a paylink-service fallback. Approval emits the
``refund.reversal.instructed`` INSTRUCTION — strictly non-custodial (A.1): no funds move here.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from typing import Any

from app.clawback.coordinator import ClawbackCoordinator
from app.config import Settings
from app.db.models import RefundRow
from app.db.repositories import RefundRepository
from app.domain.models import RefundState, is_partial_refund, refund_can_transition
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from app.events.publisher import Publisher
from app.ledger.poster import LedgerPoster
from app.logging import get_logger
from app.paylinks.client import PaylinksClient
from app.payments.client import PaymentsClient
from app.reversal.port import RailReversalRegistry

log = get_logger("refund.service")

_Commit = Callable[[], Awaitable[None]]


class RefundService:
    def __init__(
        self,
        repo: RefundRepository,
        commit: _Commit,
        publisher: Publisher,
        settings: Settings,
        payments: PaymentsClient,
        paylinks: PaylinksClient,
        reversal: RailReversalRegistry,
        clawback: ClawbackCoordinator,
        ledger: LedgerPoster,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._settings = settings
        self._payments = payments
        self._paylinks = paylinks
        self._reversal = reversal
        self._clawback = clawback
        self._ledger = ledger

    # ── helpers ──
    async def _emit(self, entity_id: str, name: str, payload: dict[str, Any]) -> None:
        await self._repo.add_event(entity_id, name, payload)
        await self._publisher.publish(name, payload)

    @staticmethod
    def _payload(row: RefundRow) -> dict[str, Any]:
        return {
            "refund_id": str(row.refund_id),
            "payment_id": row.payment_id,
            "paylink_id": row.paylink_id,
            "rail": row.rail,
            "amount_minor": int(row.amount_minor),
            "currency": row.currency,
            "is_partial": row.is_partial,
            "org_id": str(row.org_id) if row.org_id else None,
        }

    async def _resolve_original_amount(self, paylink_id: str) -> int | None:
        """The authoritative original amount: verified_paylinks projection, then paylink-service."""
        vp = await self._repo.get_verified_paylink(paylink_id)
        if vp is not None and vp.amount_minor is not None:
            return int(vp.amount_minor)
        amt = await self._paylinks.get_amount(paylink_id)
        return amt.amount_minor if amt is not None else None

    # ── reads ──
    async def get(self, refund_id: uuid.UUID) -> RefundRow:
        row = await self._repo.get_refund(refund_id)
        if row is None:
            raise AppError(ErrorCode.REFUND_NOT_FOUND, "refund not found")
        return row

    async def list_by_payment(self, payment_id: str) -> list[RefundRow]:
        return await self._repo.list_refunds_by_payment(payment_id)

    # ── request ──
    async def request_refund(
        self,
        *,
        payment_id: str,
        amount_minor: int,
        currency: str | None,
        reason: str | None,
        requested_by: str,
        org_id: uuid.UUID | None,
        merchant_id: uuid.UUID | None,
    ) -> RefundRow:
        payment = await self._payments.get(payment_id)
        if payment is None:
            raise AppError(ErrorCode.PAYMENT_NOT_FOUND, "payment not found")
        if payment.status != "SETTLED":
            raise AppError(
                ErrorCode.PAYMENT_NOT_SETTLED,
                "only a settled payment can be refunded",
                details={"status": payment.status},
            )

        original = await self._resolve_original_amount(payment.paylink_id)
        if original is None and self._settings.amount_validation == "strict":
            raise AppError(
                ErrorCode.AMOUNT_SOURCE_UNAVAILABLE,
                "could not resolve the original payment amount",
                details={"paylink_id": payment.paylink_id},
            )
        if original is not None:
            already = await self._repo.active_refund_total(payment_id)
            if already + amount_minor > original:
                raise AppError(
                    ErrorCode.REFUND_EXCEEDS_REMAINING,
                    "refund exceeds the remaining refundable amount",
                    details={"original": original, "already_refunded": already},
                )

        currency = (currency or self._settings.default_currency).upper()
        row = RefundRow(
            refund_id=uuid.uuid4(),
            payment_id=payment_id,
            paylink_id=payment.paylink_id,
            rail=payment.rail,
            merchant_id=merchant_id,
            org_id=org_id,
            requested_by=requested_by,
            amount_minor=Decimal(amount_minor),
            currency=currency,
            reason=reason,
            state=RefundState.REQUESTED.value,
            is_partial=is_partial_refund(amount_minor, original),
        )
        await self._repo.insert_refund(row)
        await self._emit(str(row.refund_id), ev.REFUND_REQUESTED, self._payload(row))
        await self._commit()
        log.info("refund_requested", refund_id=str(row.refund_id), payment_id=payment_id)
        return row

    # ── transitions ──
    def _require_transition(self, row: RefundRow, target: RefundState) -> None:
        if not refund_can_transition(RefundState(row.state), target):
            raise AppError(
                ErrorCode.INVALID_STATE_TRANSITION,
                f"cannot move refund from {row.state} to {target.value}",
                details={"from": row.state, "to": target.value},
            )

    async def approve(self, refund_id: uuid.UUID, *, approved_by: str) -> RefundRow:
        row = await self.get(refund_id)
        self._require_transition(row, RefundState.PROCESSING)
        reversal = self._reversal.for_rail(row.rail)
        reversal_ref = await reversal.instruct(
            refund_id=str(row.refund_id),
            rail=row.rail,
            amount_minor=int(row.amount_minor),
            currency=row.currency,
            paylink_id=row.paylink_id,
            payment_id=row.payment_id,
        )
        row.state = RefundState.PROCESSING.value
        row.approved_by = approved_by
        row.reversal_ref = reversal_ref
        row.updated_at = datetime.now(UTC)
        payload = self._payload(row)
        await self._emit(
            str(row.refund_id), ev.REFUND_APPROVED, {**payload, "approved_by": approved_by}
        )
        await self._emit(str(row.refund_id), ev.REFUND_REVERSAL_INSTRUCTED, payload)
        await self._emit(str(row.refund_id), ev.REFUND_PROCESSING, payload)
        await self._commit()
        log.info("refund_approved", refund_id=str(row.refund_id))
        return row

    async def reject(self, refund_id: uuid.UUID, *, rejected_by: str) -> RefundRow:
        row = await self.get(refund_id)
        self._require_transition(row, RefundState.REJECTED)
        row.state = RefundState.REJECTED.value
        row.approved_by = rejected_by
        row.updated_at = datetime.now(UTC)
        await self._emit(
            str(row.refund_id),
            ev.REFUND_REJECTED,
            {**self._payload(row), "rejected_by": rejected_by},
        )
        await self._commit()
        log.info("refund_rejected", refund_id=str(row.refund_id))
        return row

    async def complete(self, refund_id: uuid.UUID) -> RefundRow:
        """Confirm the reversal landed (rail callback in prod; the sweeper in dev-simulate)."""
        row = await self.get(refund_id)
        self._require_transition(row, RefundState.COMPLETED)
        row.state = RefundState.COMPLETED.value
        row.updated_at = datetime.now(UTC)
        if self._settings.ledger_posting_enabled:
            await self._ledger.post_refund(
                refund_id=str(row.refund_id),
                payment_id=row.payment_id,
                amount_minor=int(row.amount_minor),
                currency=row.currency,
            )
        await self._emit(str(row.refund_id), ev.REFUND_COMPLETED, self._payload(row))
        await self._commit()
        log.info("refund_completed", refund_id=str(row.refund_id))
        return row

    async def fail(self, refund_id: uuid.UUID, *, reason: str) -> RefundRow:
        row = await self.get(refund_id)
        self._require_transition(row, RefundState.FAILED)
        row.state = RefundState.FAILED.value
        row.failure_reason = reason
        row.updated_at = datetime.now(UTC)
        await self._emit(
            str(row.refund_id), ev.REFUND_FAILED, {**self._payload(row), "failure_reason": reason}
        )
        await self._commit()
        log.info("refund_failed", refund_id=str(row.refund_id), reason=reason)
        return row

    # ── sweeper (dev simulate) ──
    async def simulate_due_completions(self, now: datetime) -> int:
        """Advance PROCESSING refunds older than the simulate threshold to COMPLETED (dev/demo)."""
        cutoff = now - timedelta(seconds=self._settings.simulate_complete_after_seconds)
        rows = await self._repo.list_processing_refunds_before(cutoff)
        for row in rows:
            await self.complete(row.refund_id)
        return len(rows)
