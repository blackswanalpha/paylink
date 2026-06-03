"""Pydantic request/response models — the wire contract.

Money is integer **minor units** everywhere (``gross``, fees, totals). FX rates are serialized as
strings (lossless — no float rounding). ``currency`` codes are 3-letter ISO, upper-cased.
"""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field, field_validator


def _upper3(v: str) -> str:
    v = v.strip().upper()
    if len(v) != 3 or not v.isalpha():
        raise ValueError("currency must be a 3-letter ISO code")
    return v


# ── requests ──
class QuoteRequest(BaseModel):
    merchant_id: uuid.UUID
    gross: int = Field(ge=1, description="requested amount in integer minor units")
    currency: str = Field(description="3-letter ISO currency of `gross`")
    settle_currency: str | None = Field(
        default=None, description="currency fees/net are expressed in (defaults to `currency`)"
    )
    rails: list[str] = Field(min_length=1, description="rails to quote (mpesa|card|bank|crypto)")
    tiers: list[str] | None = Field(
        default=None, description="tiers to quote (defaults to the merchant's tier)"
    )

    @field_validator("currency")
    @classmethod
    def _v_currency(cls, v: str) -> str:
        return _upper3(v)

    @field_validator("settle_currency")
    @classmethod
    def _v_settle(cls, v: str | None) -> str | None:
        return _upper3(v) if v is not None else None

    @field_validator("rails")
    @classmethod
    def _v_rails(cls, v: list[str]) -> list[str]:
        seen: list[str] = []
        for r in v:
            rr = r.strip().lower()
            if rr and rr not in seen:
                seen.append(rr)
        if not seen:
            raise ValueError("at least one rail is required")
        return seen


class AccrualRequest(BaseModel):
    merchant_id: uuid.UUID
    amount: int = Field(ge=0, description="realized platform fee in integer minor units")
    currency: str
    source_ref: str = Field(min_length=1, max_length=200)
    occurred_at: datetime
    quote_id: uuid.UUID | None = None

    @field_validator("currency")
    @classmethod
    def _v_currency(cls, v: str) -> str:
        return _upper3(v)


class RunRequest(BaseModel):
    period: str = Field(description="billing period 'YYYY-MM'")
    merchant_id: uuid.UUID | None = None


# ── responses ──
class QuoteBreakdown(BaseModel):
    quote_id: str
    tier: str
    rail: str
    gross: int
    currency: str
    gross_settled: int
    settle_currency: str
    platform_fee: int
    rail_fee: int
    net: int
    fx: dict[str, Any] | None
    breakdown: dict[str, Any]


class QuoteResponse(BaseModel):
    quotes: list[QuoteBreakdown]


class FxRateResponse(BaseModel):
    base: str
    quote: str
    rate: str
    source: str
    fetched_at: datetime


class FxRatesResponse(BaseModel):
    rates: list[FxRateResponse]


class TierResponse(BaseModel):
    tier: str
    display_name: str
    platform_pct_bps: int
    platform_fixed: int
    fixed_currency: str
    active: bool


class TiersResponse(BaseModel):
    tiers: list[TierResponse]


class MerchantRailFeeResponse(BaseModel):
    rail: str
    pct_bps: int
    fixed: int
    fixed_currency: str


class MerchantPricingResponse(BaseModel):
    merchant_id: str
    org_id: str | None
    tier: str
    display_name: str
    platform_pct_bps: int
    platform_fixed: int
    fixed_currency: str
    rail_fees: list[MerchantRailFeeResponse]
    effective_at: datetime


class AccrualResponse(BaseModel):
    accrual_id: int
    accepted: bool


class GeneratedInvoiceResponse(BaseModel):
    invoice_id: str
    merchant_id: str
    currency: str
    total_fee: int
    line_count: int


class RunResponse(BaseModel):
    period: str
    generated: list[GeneratedInvoiceResponse]
    skipped_existing: int
