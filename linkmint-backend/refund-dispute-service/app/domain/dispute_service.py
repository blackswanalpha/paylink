"""Dispute domain service: HMAC webhook intake → evidence window → submit → WON/LOST/EXPIRED.

A rail/PSP raises a dispute via the signed webhook (intake); the merchant gathers evidence within
the
rail-imposed window and submits; the rail's resolution arrives on the same webhook. A loss (or an
expired window) requests a clawback from the merchant's next payout (work23) via the coordinator
seam. Non-custodial (A.1): no funds move — the clawback is a published instruction.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from typing import Any

from app.clawback.coordinator import ClawbackCoordinator
from app.config import Settings
from app.db.models import DisputeEvidenceRow, DisputeRow
from app.db.repositories import RefundRepository
from app.domain.models import DISPUTE_LOSS_STATES, DisputeState, dispute_can_transition
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from app.events.publisher import Publisher
from app.logging import get_logger

log = get_logger("dispute.service")

_Commit = Callable[[], Awaitable[None]]

# Webhook payload kinds (tolerant of a short or dotted form).
_OPENED_KINDS = frozenset({"dispute.opened", "opened"})
_RESOLVED_KINDS = frozenset({"dispute.resolved", "resolved"})


@dataclass(frozen=True)
class IntakeResult:
    action: str  # opened|opened_replay|resolved|resolved_noop|ignored
    dispute_id: str | None


def _parse_dt(value: Any) -> datetime | None:
    if not isinstance(value, str):
        return None
    try:
        dt = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    return dt if dt.tzinfo else dt.replace(tzinfo=UTC)


def _coerce_amount(value: Any) -> Decimal | None:
    if value is None:
        return None
    try:
        return Decimal(str(value))
    except (ValueError, ArithmeticError):
        return None


class DisputeService:
    def __init__(
        self,
        repo: RefundRepository,
        commit: _Commit,
        publisher: Publisher,
        settings: Settings,
        clawback: ClawbackCoordinator,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._settings = settings
        self._clawback = clawback

    async def _emit(self, entity_id: str, name: str, payload: dict[str, Any]) -> None:
        await self._repo.add_event(entity_id, name, payload)
        await self._publisher.publish(name, payload)

    @staticmethod
    def _payload(row: DisputeRow) -> dict[str, Any]:
        return {
            "dispute_id": str(row.dispute_id),
            "provider": row.provider,
            "provider_dispute_id": row.provider_dispute_id,
            "payment_id": row.payment_id,
            "paylink_id": row.paylink_id,
            "rail": row.rail,
            "amount_minor": int(row.amount_minor) if row.amount_minor is not None else None,
            "currency": row.currency,
            "org_id": str(row.org_id) if row.org_id else None,
        }

    # ── reads ──
    async def get(self, dispute_id: uuid.UUID) -> tuple[DisputeRow, list[DisputeEvidenceRow]]:
        row = await self._repo.get_dispute(dispute_id)
        if row is None:
            raise AppError(ErrorCode.DISPUTE_NOT_FOUND, "dispute not found")
        evidence = await self._repo.list_evidence(dispute_id)
        return row, evidence

    # ── webhook intake (HMAC-verified upstream in the route) ──
    async def intake(self, *, provider: str, body: dict[str, Any]) -> IntakeResult:
        kind = str(body.get("kind", "")).strip()
        if kind in _OPENED_KINDS:
            return await self._open(provider, body)
        if kind in _RESOLVED_KINDS:
            return await self._resolve_from_webhook(provider, body)
        log.warning("dispute_webhook_ignored", provider=provider, kind=kind)
        return IntakeResult(action="ignored", dispute_id=None)

    async def _open(self, provider: str, body: dict[str, Any]) -> IntakeResult:
        provider_dispute_id = body.get("provider_dispute_id") or body.get("dispute_id")
        if not provider_dispute_id:
            raise AppError(ErrorCode.INVALID_PAYLOAD, "missing provider_dispute_id")
        due = _parse_dt(body.get("evidence_due_at")) or (
            datetime.now(UTC) + timedelta(hours=self._settings.dispute_evidence_window_hours)
        )
        org_id = body.get("org_id")
        merchant_id = body.get("merchant_id")
        row = DisputeRow(
            dispute_id=uuid.uuid4(),
            provider=provider,
            provider_dispute_id=str(provider_dispute_id),
            payment_id=str(body["payment_id"]) if body.get("payment_id") else None,
            paylink_id=str(body["paylink_id"]) if body.get("paylink_id") else None,
            rail=str(body.get("rail") or provider),
            merchant_id=uuid.UUID(str(merchant_id)) if merchant_id else None,
            org_id=uuid.UUID(str(org_id)) if org_id else None,
            amount_minor=_coerce_amount(body.get("amount_minor")),
            currency=body.get("currency"),
            reason_code=body.get("reason_code"),
            state=DisputeState.OPEN.value,
            evidence_due_at=due,
            clawback_requested=False,
        )
        inserted = await self._repo.insert_dispute_if_absent(row)
        if not inserted:
            # Anti-replay (A.7): a re-delivered open is a no-op; return the existing dispute id.
            existing = await self._repo.get_dispute_by_provider_ref(
                provider, str(provider_dispute_id)
            )
            await self._commit()
            return IntakeResult(
                action="opened_replay",
                dispute_id=str(existing.dispute_id) if existing else None,
            )
        await self._emit(
            str(row.dispute_id),
            ev.DISPUTE_OPENED,
            {**self._payload(row), "evidence_due_at": due.isoformat()},
        )
        await self._commit()
        log.info("dispute_opened", dispute_id=str(row.dispute_id), provider=provider)
        return IntakeResult(action="opened", dispute_id=str(row.dispute_id))

    async def _resolve_from_webhook(self, provider: str, body: dict[str, Any]) -> IntakeResult:
        provider_dispute_id = body.get("provider_dispute_id") or body.get("dispute_id")
        if not provider_dispute_id:
            raise AppError(ErrorCode.INVALID_PAYLOAD, "missing provider_dispute_id")
        outcome = str(body.get("outcome", "")).lower()
        if outcome not in ("won", "lost"):
            raise AppError(ErrorCode.INVALID_PAYLOAD, "resolution outcome must be 'won' or 'lost'")
        dispute = await self._repo.get_dispute_by_provider_ref(provider, str(provider_dispute_id))
        if dispute is None:
            raise AppError(ErrorCode.DISPUTE_NOT_FOUND, "dispute not found")
        target = DisputeState.WON if outcome == "won" else DisputeState.LOST
        applied = await self._apply_resolution(dispute, target)
        return IntakeResult(
            action="resolved" if applied else "resolved_noop", dispute_id=str(dispute.dispute_id)
        )

    async def _apply_resolution(self, dispute: DisputeRow, target: DisputeState) -> bool:
        """Transition a dispute to a terminal outcome. Idempotent: a repeated resolution is a
        no-op (first outcome wins). Returns True when a transition was applied."""
        current = DisputeState(dispute.state)
        if current in (DisputeState.WON, DisputeState.LOST, DisputeState.EXPIRED):
            if current != target:
                log.warning(
                    "dispute_resolution_conflict",
                    dispute_id=str(dispute.dispute_id),
                    current=current.value,
                    attempted=target.value,
                )
            return False
        if not dispute_can_transition(current, target):
            raise AppError(
                ErrorCode.INVALID_STATE_TRANSITION,
                f"cannot move dispute from {current.value} to {target.value}",
                details={"from": current.value, "to": target.value},
            )
        await self._finalize(dispute, target, reason="dispute_lost")
        return True

    async def _finalize(self, dispute: DisputeRow, target: DisputeState, *, reason: str) -> None:
        dispute.state = target.value
        dispute.resolved_at = datetime.now(UTC)
        dispute.updated_at = datetime.now(UTC)
        name = {
            DisputeState.WON: ev.DISPUTE_WON,
            DisputeState.LOST: ev.DISPUTE_LOST,
            DisputeState.EXPIRED: ev.DISPUTE_EXPIRED,
        }[target]
        await self._emit(
            str(dispute.dispute_id),
            name,
            {
                **self._payload(dispute),
                "outcome": target.value,
                "resolved_at": dispute.resolved_at.isoformat(),
            },
        )
        if target in DISPUTE_LOSS_STATES:
            dispute.clawback_requested = True
            await self._clawback.request_clawback(
                self._repo,
                dispute_id=str(dispute.dispute_id),
                payment_id=dispute.payment_id,
                paylink_id=dispute.paylink_id,
                merchant_id=str(dispute.merchant_id) if dispute.merchant_id else None,
                org_id=str(dispute.org_id) if dispute.org_id else None,
                amount_minor=(
                    int(dispute.amount_minor) if dispute.amount_minor is not None else None
                ),
                currency=dispute.currency,
                reason=reason,
            )
        await self._commit()

    # ── evidence ──
    async def add_evidence(
        self,
        *,
        dispute_id: uuid.UUID,
        kind: str,
        summary: str | None,
        payload: dict[str, Any] | None,
        external_ref: str | None,
        submitted_by: str,
    ) -> DisputeEvidenceRow:
        dispute = await self._repo.get_dispute(dispute_id)
        if dispute is None:
            raise AppError(ErrorCode.DISPUTE_NOT_FOUND, "dispute not found")
        if dispute.state != DisputeState.OPEN.value:
            raise AppError(
                ErrorCode.EVIDENCE_WINDOW_CLOSED,
                "evidence can only be added while the dispute is OPEN",
                details={"state": dispute.state},
            )
        if datetime.now(UTC) >= dispute.evidence_due_at:
            raise AppError(
                ErrorCode.EVIDENCE_WINDOW_CLOSED,
                "the evidence window has closed",
                details={"evidence_due_at": dispute.evidence_due_at.isoformat()},
            )
        row = DisputeEvidenceRow(
            evidence_id=uuid.uuid4(),
            dispute_id=dispute_id,
            kind=kind,
            summary=summary,
            payload=payload or {},
            external_ref=external_ref,
            submitted_by=submitted_by,
        )
        await self._repo.insert_evidence(row)
        await self._emit(
            str(dispute_id),
            ev.DISPUTE_EVIDENCE_ADDED,
            {"dispute_id": str(dispute_id), "evidence_id": str(row.evidence_id), "kind": kind},
        )
        await self._commit()
        log.info("dispute_evidence_added", dispute_id=str(dispute_id))
        return row

    async def submit(self, dispute_id: uuid.UUID, *, submitted_by: str) -> DisputeRow:
        dispute = await self._repo.get_dispute(dispute_id)
        if dispute is None:
            raise AppError(ErrorCode.DISPUTE_NOT_FOUND, "dispute not found")
        if not dispute_can_transition(DisputeState(dispute.state), DisputeState.SUBMITTED):
            raise AppError(
                ErrorCode.INVALID_STATE_TRANSITION,
                f"cannot submit a dispute in state {dispute.state}",
                details={"from": dispute.state, "to": DisputeState.SUBMITTED.value},
            )
        dispute.state = DisputeState.SUBMITTED.value
        dispute.submitted_at = datetime.now(UTC)
        dispute.updated_at = datetime.now(UTC)
        evidence_count = await self._repo.count_evidence(dispute_id)
        await self._emit(
            str(dispute_id),
            ev.DISPUTE_SUBMITTED,
            {
                **self._payload(dispute),
                "evidence_count": evidence_count,
                "submitted_by": submitted_by,
            },
        )
        await self._commit()
        log.info("dispute_submitted", dispute_id=str(dispute_id), evidence_count=evidence_count)
        return dispute

    # ── sweeper ──
    async def expire_due(self, now: datetime) -> int:
        """Expire OPEN disputes past their evidence window → EXPIRED + clawback (treated as a
        loss)."""
        rows = await self._repo.list_open_disputes_due_before(now)
        for dispute in rows:
            await self._finalize(dispute, DisputeState.EXPIRED, reason="dispute_expired")
        return len(rows)
