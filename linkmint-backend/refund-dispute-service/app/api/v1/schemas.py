"""Request/response models for the v1 surface.

Money is integer minor units. ``payment_id`` / ``paylink_id`` are opaque ids; ``org_id`` /
``merchant_id`` are opaque refs to identity / merchant-onboarding (no cross-schema FK). Nothing
fund-moving is stored or returned (A.1).
"""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field

from app.db.models import DisputeEvidenceRow, DisputeRow, RefundRow


# ── refunds ──
class CreateRefundRequest(BaseModel):
    payment_id: str = Field(min_length=1)
    amount_minor: int = Field(gt=0, description="refund amount in integer minor units")
    currency: str | None = Field(default=None, description="ISO 4217; defaults per config")
    reason: str | None = None
    org_id: str | None = Field(default=None, description="owning org (membership is verified)")
    merchant_id: str | None = None


class RefundView(BaseModel):
    refund_id: str
    payment_id: str
    paylink_id: str
    rail: str
    org_id: str | None
    merchant_id: str | None
    amount_minor: int
    currency: str
    reason: str | None
    state: str
    is_partial: bool
    approved_by: str | None
    failure_reason: str | None
    reversal_ref: str | None
    created_at: datetime
    updated_at: datetime

    @classmethod
    def from_row(cls, row: RefundRow) -> RefundView:
        return cls(
            refund_id=str(row.refund_id),
            payment_id=row.payment_id,
            paylink_id=row.paylink_id,
            rail=row.rail,
            org_id=str(row.org_id) if row.org_id else None,
            merchant_id=str(row.merchant_id) if row.merchant_id else None,
            amount_minor=int(row.amount_minor),
            currency=row.currency,
            reason=row.reason,
            state=row.state,
            is_partial=row.is_partial,
            approved_by=row.approved_by,
            failure_reason=row.failure_reason,
            reversal_ref=row.reversal_ref,
            created_at=row.created_at,
            updated_at=row.updated_at,
        )


class RefundListResponse(BaseModel):
    refunds: list[RefundView]


# ── disputes ──
class AddEvidenceRequest(BaseModel):
    kind: str = Field(min_length=1, description="receipt|tracking|note|...")
    summary: str | None = None
    payload: dict = Field(default_factory=dict)
    external_ref: str | None = None


class EvidenceView(BaseModel):
    evidence_id: str
    kind: str
    summary: str | None
    external_ref: str | None
    submitted_by: str
    created_at: datetime

    @classmethod
    def from_row(cls, row: DisputeEvidenceRow) -> EvidenceView:
        return cls(
            evidence_id=str(row.evidence_id),
            kind=row.kind,
            summary=row.summary,
            external_ref=row.external_ref,
            submitted_by=row.submitted_by,
            created_at=row.created_at,
        )


class DisputeView(BaseModel):
    dispute_id: str
    provider: str
    provider_dispute_id: str
    payment_id: str | None
    paylink_id: str | None
    rail: str
    org_id: str | None
    merchant_id: str | None
    amount_minor: int | None
    currency: str | None
    reason_code: str | None
    state: str
    evidence_due_at: datetime
    submitted_at: datetime | None
    resolved_at: datetime | None
    clawback_requested: bool
    evidence: list[EvidenceView] = Field(default_factory=list)
    created_at: datetime

    @classmethod
    def from_row(
        cls, row: DisputeRow, evidence: list[DisputeEvidenceRow] | None = None
    ) -> DisputeView:
        return cls(
            dispute_id=str(row.dispute_id),
            provider=row.provider,
            provider_dispute_id=row.provider_dispute_id,
            payment_id=row.payment_id,
            paylink_id=row.paylink_id,
            rail=row.rail,
            org_id=str(row.org_id) if row.org_id else None,
            merchant_id=str(row.merchant_id) if row.merchant_id else None,
            amount_minor=int(row.amount_minor) if row.amount_minor is not None else None,
            currency=row.currency,
            reason_code=row.reason_code,
            state=row.state,
            evidence_due_at=row.evidence_due_at,
            submitted_at=row.submitted_at,
            resolved_at=row.resolved_at,
            clawback_requested=row.clawback_requested,
            evidence=[EvidenceView.from_row(e) for e in (evidence or [])],
            created_at=row.created_at,
        )


class WebhookResponse(BaseModel):
    status: str = "ok"
    action: str
    dispute_id: str | None = None
