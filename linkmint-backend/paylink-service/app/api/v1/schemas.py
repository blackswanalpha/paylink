"""Request/response models. Deliberately **rail-agnostic** (invariant A.4): no mpesa/card/bank
fields — ``metadata`` is opaque JSON and carries nothing fund-moving (A.1)."""

from __future__ import annotations

import re
from datetime import UTC, datetime
from typing import Any, Literal

from pydantic import BaseModel, Field, field_validator

from app.db.models import PayLinkRow

_ADDR_RE = re.compile(r"^0x[0-9a-fA-F]{40}$")


class CreatePayLinkRequest(BaseModel):
    receiver: str = Field(description="20-byte chain address, 0x-prefixed hex")
    amount: int = Field(gt=0, description="amount in integer minor units")
    currency: str | None = Field(default=None, description="ISO 4217 or PLN; defaults per config")
    expiry: datetime
    usage: Literal["single", "multi"] = "single"
    metadata: dict[str, Any] | None = None
    rules: Any | None = None

    @field_validator("receiver")
    @classmethod
    def _validate_receiver(cls, v: str) -> str:
        if not _ADDR_RE.match(v):
            raise ValueError("must be a 0x-prefixed 20-byte hex address")
        return v.lower()

    @field_validator("expiry")
    @classmethod
    def _validate_expiry(cls, v: datetime) -> datetime:
        if v.tzinfo is None:
            v = v.replace(tzinfo=UTC)
        if v <= datetime.now(UTC):
            raise ValueError("expiry must be in the future")
        return v


class PayLinkResponse(BaseModel):
    pl_id: str
    creator: str
    receiver: str
    owner: str
    amount: int
    currency: str
    status: str
    expiry: datetime
    usage: str
    vote_count: int
    chain_tx_hash: str | None
    created_at: datetime
    updated_at: datetime
    verified_at: datetime | None

    @classmethod
    def from_row(cls, row: PayLinkRow) -> PayLinkResponse:
        return cls(
            pl_id=row.pl_id,
            creator=row.creator_addr,
            receiver=row.receiver_addr,
            owner=row.owner_addr,
            amount=int(row.amount),
            currency=row.currency,
            status=row.status,
            expiry=row.expiry,
            usage=row.usage,
            vote_count=row.vote_count,
            chain_tx_hash=row.chain_tx_hash,
            created_at=row.created_at,
            updated_at=row.updated_at,
            verified_at=row.verified_at,
        )


class CreatePayLinkResponse(BaseModel):
    pl_id: str
    status: str
    created_at: datetime
    chain_tx_hash: str | None


class CancelResponse(BaseModel):
    pl_id: str
    status: str


class PayLinkListResponse(BaseModel):
    items: list[PayLinkResponse]
    next_cursor: str | None
