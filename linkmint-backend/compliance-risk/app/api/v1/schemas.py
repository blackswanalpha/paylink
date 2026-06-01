"""Request/response models for the compliance-risk API.

INVARIANT: no response model ever exposes raw PII or the encrypted ``provider_ref``. KYC status is
surfaced by tier + latest risk score + open-flag summaries only.

The ``/v1/risk/evaluate`` request/response shapes are a FIXED contract consumed by paylink-service —
field names must not drift.
"""

from __future__ import annotations

from decimal import Decimal

from pydantic import BaseModel, Field


# ── KYC sessions ──
class CreateKycSessionRequest(BaseModel):
    user_id: str
    tier_requested: int = Field(ge=1, le=2)


class CreateKycSessionResponse(BaseModel):
    session_id: str
    provider_url: str


# ── KYC callbacks ──
class CallbackResponse(BaseModel):
    ok: bool = True


# ── Compliance status ──
class ComplianceFlag(BaseModel):
    kind: str
    severity: str
    raised_at: str | None


class ComplianceStatusResponse(BaseModel):
    user_id: str
    kyc_tier: int
    risk_score: float | None
    flags: list[ComplianceFlag]


# ── Risk evaluate (INTERNAL — fixed contract consumed by paylink-service) ──
class RiskEvaluateRequest(BaseModel):
    # Bounds harden the internal endpoint against a direct caller (defense-in-depth): a negative
    # ``amount`` must NOT defeat the LOW_KYC/AML hard rules, and unbounded strings must not amplify
    # into risk_scores.context / flags.payload / logs. The gate caller (paylink) already sends
    # amount>0, so these never reject a legitimate gate request.
    user_id: str = Field(max_length=64)
    action: str = Field(min_length=1, max_length=64)
    amount: Decimal | None = Field(default=None, ge=0, le=Decimal("1e15"))
    currency: str = Field(default="KES", max_length=8)
    geo: str | None = Field(default=None, max_length=8)
    registered_country: str | None = Field(default=None, max_length=8)
    context: str | None = Field(default=None, max_length=200)


class RiskReason(BaseModel):
    code: str
    detail: str


class RiskEvaluateResponse(BaseModel):
    decision: str  # allow|block|review
    score: float
    reasons: list[RiskReason]
