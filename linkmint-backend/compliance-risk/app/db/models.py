"""ORM models for the ``compliance`` schema (mirrors backendfeatures.md §2.6 DDL).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.

There are NO cross-schema FKs: ``user_id`` is an OPAQUE ref to identity.users (the owning service is
reached only by id). ``kyc_records.documents`` stores ONLY redacted metadata (an allowlist of safe
scalar keys — see :mod:`app.redaction`); raw PII (names, id numbers, document images) is never
persisted, logged, returned, or placed in an event payload.
"""

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import BigInteger, DateTime, Numeric, SmallInteger, Text, text
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class KycRecordRow(Base):
    """One row per user — the KYC tier + (encrypted) provider reference + redacted metadata."""

    __tablename__ = "kyc_records"
    __mapper_args__ = {"eager_defaults": True}

    # Opaque ref to identity.users — NO cross-schema FK. PK enforces one record per user.
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    tier: Mapped[int] = mapped_column(
        SmallInteger, nullable=False, server_default=text("0")
    )  # 0=none, 1=basic, 2=enhanced
    provider: Mapped[str | None] = mapped_column(Text, nullable=True)
    # AES-256-GCM ciphertext of the provider's session reference (KMS stand-in) — never plaintext.
    provider_ref: Mapped[str | None] = mapped_column(Text, nullable=True)
    documents: Mapped[dict[str, Any] | None] = mapped_column(
        JSONB, nullable=True
    )  # redacted metadata ONLY (allowlist)
    verified_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    expires_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class RiskScoreRow(Base):
    """Append-only audit of every risk decision (the source of the latest risk_score)."""

    __tablename__ = "risk_scores"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    context: Mapped[str] = mapped_column(Text, nullable=False)  # e.g. 'paylink.create'
    score: Mapped[Decimal] = mapped_column(Numeric(4, 3), nullable=False)  # 0.000 .. 1.000
    decision: Mapped[str] = mapped_column(Text, nullable=False)  # allow|block|review
    reasons: Mapped[list[dict[str, Any]]] = mapped_column(JSONB, nullable=False)
    evaluated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class FlagRow(Base):
    """A raised compliance flag (block/warn/info) — append-only; ``resolved_at`` set on resolve."""

    __tablename__ = "flags"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # sanctions|velocity|geo|manual
    severity: Mapped[str] = mapped_column(Text, nullable=False)  # info|warn|block
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    raised_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    resolved_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    resolution: Mapped[str | None] = mapped_column(Text, nullable=True)


class ActivityEventRow(Base):
    """Hot-path activity ledger for windowed velocity counts + the cumulative-amount AML sum.

    Fed by the ``payment.initiated`` consumer (and any future value-action signal). ``evaluate``
    deliberately does NOT append here (so a risk read never pollutes velocity).
    """

    __tablename__ = "activity_events"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    action: Mapped[str] = mapped_column(Text, nullable=False)
    amount: Mapped[Decimal | None] = mapped_column(Numeric(20, 2), nullable=True)
    currency: Mapped[str | None] = mapped_column(Text, nullable=True)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class ComplianceEventRow(Base):
    """Durable outbox — the source of truth work15 will drain onto Kafka/SQS.

    Payloads NEVER carry raw PII (only ids/tier/decision/reasons metadata).
    """

    __tablename__ = "compliance_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    subject_type: Mapped[str] = mapped_column(Text, nullable=False)  # user|risk|flag|...
    subject_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name (compliance.*)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
