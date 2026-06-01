"""ORM models for the ``merchant`` schema (mirrors backendfeatures.md §2.10 DDL).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.

FKs are within the ``merchant`` schema ONLY (no cross-schema FKs). ``org_id`` (→
identity.organizations) and ``accepted_by`` (→ identity.users) are OPAQUE UUID columns with NO
foreign key — the owning service (identity) is reached only by id. ``account_ref`` is AES-256-GCM
ciphertext (the KMS stand-in); the plaintext bank account details are never stored.
"""

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import BigInteger, DateTime, ForeignKey, Text, text
from sqlalchemy.dialects.postgresql import INET, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class MerchantRow(Base):
    __tablename__ = "merchants"
    __mapper_args__ = {"eager_defaults": True}

    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    # Opaque ref to identity.organizations — NO cross-schema FK. UNIQUE → one-merchant-per-org.
    org_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False, unique=True)
    business_name: Mapped[str] = mapped_column(Text, nullable=False)
    registration_no: Mapped[str | None] = mapped_column(Text, nullable=True)
    tax_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    country: Mapped[str] = mapped_column(Text, nullable=False)  # ISO 3166-1 alpha-2
    type: Mapped[str] = mapped_column(Text, nullable=False)  # individual|company|nonprofit
    status: Mapped[str] = mapped_column(Text, nullable=False)  # DRAFT|PENDING_VERIFICATION|ACTIVE..
    fee_tier: Mapped[str] = mapped_column(Text, nullable=False, server_default=text("'standard'"))
    onboarded_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    suspended_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    suspended_reason: Mapped[str | None] = mapped_column(Text, nullable=True)


class BankAccountRow(Base):
    __tablename__ = "bank_accounts"
    __mapper_args__ = {"eager_defaults": True}

    bank_account_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("merchant.merchants.merchant_id"), nullable=False
    )
    rail: Mapped[str] = mapped_column(Text, nullable=False)  # mpesa|swift|sepa|ach|crypto
    # KMS-encrypted (AES-256-GCM) account number / wallet — NEVER plaintext.
    account_ref: Mapped[str] = mapped_column(Text, nullable=False)
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # PENDING_VERIFY|VERIFIED|REVOKED
    verified_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class DocumentRow(Base):
    __tablename__ = "documents"
    __mapper_args__ = {"eager_defaults": True}

    document_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("merchant.merchants.merchant_id"), nullable=False
    )
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # cert_incorporation|tax_id|director_id
    s3_key: Mapped[str] = mapped_column(Text, nullable=False)  # object-store key; bytes live there
    uploaded_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    review: Mapped[dict[str, Any] | None] = mapped_column(JSONB, nullable=True)  # compliance result


class ContractRow(Base):
    __tablename__ = "contracts"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("merchant.merchants.merchant_id"), nullable=False
    )
    version: Mapped[str] = mapped_column(Text, nullable=False)
    # Opaque ref to identity.users — NO cross-schema FK.
    accepted_by: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    accepted_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    ip: Mapped[str | None] = mapped_column(INET, nullable=True)
    user_agent: Mapped[str | None] = mapped_column(Text, nullable=True)


class MerchantEventRow(Base):
    """Durable outbox — the source of truth work15 will drain onto Kafka/SQS.

    Payloads NEVER carry plaintext bank-account details (only ids/status/metadata).
    """

    __tablename__ = "merchant_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    subject_type: Mapped[str] = mapped_column(Text, nullable=False)  # merchant|bank_account|...
    subject_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name (merchant.*)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
