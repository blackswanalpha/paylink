"""Request/response models for the merchant-onboarding API.

INVARIANT: no response model ever exposes ``account_ref`` (the AES-GCM ciphertext) or the plaintext
``account_details``. Bank accounts are surfaced by id + status + rail/currency only.
"""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field

from app.domain.models import FeeTier, MerchantType, Rail, ReviewDecision


# ── onboarding ──
class OnboardRequest(BaseModel):
    org_id: str
    business_name: str = Field(min_length=1, max_length=200)
    registration_no: str | None = Field(default=None, max_length=128)
    country: str = Field(min_length=2, max_length=2)  # ISO 3166-1 alpha-2
    type: MerchantType


class OnboardResponse(BaseModel):
    merchant_id: str
    status: str


# ── full record ──
class BankAccountSummary(BaseModel):
    """Bank account surfaced by status only — never the ref/details."""

    bank_account_id: str
    rail: str
    currency: str
    status: str
    verified_at: datetime | None


class MerchantResponse(BaseModel):
    merchant_id: str
    org_id: str
    business_name: str
    registration_no: str | None
    tax_id: str | None
    country: str
    type: str
    status: str
    fee_tier: str
    onboarded_at: datetime | None
    suspended_at: datetime | None
    suspended_reason: str | None
    bank_accounts: list[BankAccountSummary]


# ── internal admin (read-only; consumed by admin-backoffice over the trusted internal surface) ──
class AdminMerchantSummary(BaseModel):
    merchant_id: str
    org_id: str
    business_name: str
    country: str
    type: str
    status: str
    fee_tier: str


class AdminMerchantListResponse(BaseModel):
    items: list[AdminMerchantSummary]


# ── documents ──
class DocumentResponse(BaseModel):
    document_id: str
    status: str  # always "UPLOADED" on success (per spec)


# ── bank accounts ──
class AddBankAccountRequest(BaseModel):
    rail: Rail
    # Plaintext bank account number / wallet — encrypted to account_ref immediately, never stored,
    # logged, returned, or emitted in plaintext.
    account_details: str = Field(min_length=1, max_length=256)
    currency: str = Field(min_length=3, max_length=3)  # ISO 4217
    country: str = Field(min_length=2, max_length=2)


class AddBankAccountResponse(BaseModel):
    bank_account_id: str
    status: str


class VerifyBankAccountRequest(BaseModel):
    # Micro-deposit amounts for ACH/SEPA (Phase 1 Kenya/MPesa = manual name-match stub).
    micro_deposit_amounts: list[int] | None = None


class VerifyBankAccountResponse(BaseModel):
    bank_account_id: str
    status: str


# ── contracts ──
class AcceptContractRequest(BaseModel):
    contract_version: str = Field(min_length=1, max_length=64)
    accepted: bool


class ContractResponse(BaseModel):
    id: int
    merchant_id: str
    version: str
    accepted_by: str
    accepted_at: datetime
    ip: str | None
    user_agent: str | None


class ContractListResponse(BaseModel):
    items: list[ContractResponse]


# ── fee tier ──
class FeeTierResponse(BaseModel):
    merchant_id: str
    tier: str
    effective_at: datetime


class UpdateFeeTierRequest(BaseModel):
    tier: FeeTier


# ── internal decision ──
class DecisionRequest(BaseModel):
    decision: ReviewDecision
    reason: str | None = Field(default=None, max_length=512)


class DecisionResponse(BaseModel):
    merchant_id: str
    status: str
