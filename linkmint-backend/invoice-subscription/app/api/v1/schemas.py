"""Pydantic request/response models for ``/v1/invoices`` (the wire contract).

Money is integer minor units (``unit_price`` and all totals are ints). ``quantity`` and ``tax_rate``
are decimals serialized as strings on the wire (lossless — avoids float rounding). ``payee_addr`` is
a 0x-prefixed 20-byte hex address (validated + lowercased), which becomes the PayLink receiver.
"""

from __future__ import annotations

import re
import uuid
from datetime import UTC, datetime
from decimal import Decimal

from pydantic import BaseModel, Field, field_validator

_ADDR_RE = re.compile(r"^0x[0-9a-f]{40}$")


# ── requests ──
class InvoiceLineInput(BaseModel):
    description: str = Field(min_length=1, max_length=1000)
    quantity: Decimal = Field(gt=0, max_digits=20, decimal_places=4)
    unit_price: int = Field(ge=0, description="unit price in integer minor units")
    tax_rate: Decimal = Field(
        default=Decimal(0), ge=0, le=Decimal("9.9999"), max_digits=5, decimal_places=4
    )


class CreateInvoiceRequest(BaseModel):
    customer_id: uuid.UUID | None = None
    payee_addr: str = Field(description="0x-prefixed 20-byte hex address of the payee (merchant)")
    currency: str | None = Field(default=None, min_length=3, max_length=10)
    lines: list[InvoiceLineInput] = Field(min_length=1)
    due_at: datetime

    @field_validator("payee_addr")
    @classmethod
    def _validate_addr(cls, v: str) -> str:
        norm = v.strip().lower()
        if not _ADDR_RE.match(norm):
            raise ValueError("payee_addr must be a 0x-prefixed 20-byte hex address")
        return norm

    @field_validator("due_at")
    @classmethod
    def _validate_due(cls, v: datetime) -> datetime:
        if v.tzinfo is None:
            v = v.replace(tzinfo=UTC)
        if v <= datetime.now(UTC):
            raise ValueError("due_at must be in the future")
        return v


# ── responses ──
class CreateInvoiceResponse(BaseModel):
    invoice_id: str
    pl_id: str | None
    status: str


class FinalizeResponse(BaseModel):
    invoice_id: str
    status: str
    pl_id: str


class VoidResponse(BaseModel):
    invoice_id: str
    status: str


class InvoiceLineResponse(BaseModel):
    description: str
    quantity: str
    unit_price: int
    total: int
    tax_rate: str


class InvoiceSummaryResponse(BaseModel):
    invoice_id: str
    merchant_id: str
    customer_id: str | None
    payee_addr: str
    pl_id: str | None
    currency: str
    subtotal: int
    tax: int
    total: int
    status: str
    due_at: datetime
    paid_at: datetime | None
    created_at: datetime
    updated_at: datetime


class InvoiceResponse(InvoiceSummaryResponse):
    lines: list[InvoiceLineResponse]


class InvoiceListResponse(BaseModel):
    items: list[InvoiceSummaryResponse]
    limit: int
    offset: int
    count: int
