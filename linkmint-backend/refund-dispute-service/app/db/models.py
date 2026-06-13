"""ORM models for the ``refund`` schema (work22 — the work item body is the authoritative scope;
``backendfeatures.md`` §2.9 is not in the working tree).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.

There are NO cross-schema FKs: ``merchant_id`` / ``org_id`` / ``payment_id`` / ``paylink_id`` are
OPAQUE refs to other services / the chain. Money is stored as integer minor units in
``NUMERIC(38,0)``. This service is strictly non-custodial (rules.md A.1): it records refund/dispute
state and emits INSTRUCTIONS — it never holds or moves funds.
"""

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import (
    BigInteger,
    Boolean,
    DateTime,
    ForeignKey,
    Numeric,
    Text,
    UniqueConstraint,
    text,
)
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base

# State value sets (mirrored by the CHECK constraints in the migration + the pure state machine).
REFUND_STATES = ("REQUESTED", "APPROVED", "REJECTED", "PROCESSING", "COMPLETED", "FAILED")
DISPUTE_STATES = ("OPEN", "SUBMITTED", "WON", "LOST", "EXPIRED")


class RefundRow(Base):
    """A sender/merchant-initiated refund and its lifecycle state."""

    __tablename__ = "refunds"
    __mapper_args__ = {"eager_defaults": True}

    refund_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    payment_id: Mapped[str] = mapped_column(Text, nullable=False)  # opaque (payment-orchestrator)
    paylink_id: Mapped[str] = mapped_column(Text, nullable=False)  # opaque (chain PayLink id)
    rail: Mapped[str] = mapped_column(Text, nullable=False)  # mpesa|card|bank|crypto (opaque, A.4)
    merchant_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    org_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    requested_by: Mapped[str] = mapped_column(Text, nullable=False)  # user_id from the token
    amount_minor: Mapped[int] = mapped_column(Numeric(38, 0), nullable=False)
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    reason: Mapped[str | None] = mapped_column(Text, nullable=True)
    state: Mapped[str] = mapped_column(Text, nullable=False)
    is_partial: Mapped[bool] = mapped_column(Boolean, nullable=False)
    approved_by: Mapped[str | None] = mapped_column(Text, nullable=True)
    failure_reason: Mapped[str | None] = mapped_column(Text, nullable=True)
    reversal_ref: Mapped[str | None] = mapped_column(Text, nullable=True)  # rail/instruction handle
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class DisputeRow(Base):
    """A rail-initiated dispute/chargeback and its lifecycle state.

    ``UNIQUE(provider, provider_dispute_id)`` is the webhook anti-replay key (A.7): a replayed
    ``dispute.opened`` is a no-op INSERT.
    """

    __tablename__ = "disputes"
    __mapper_args__ = {"eager_defaults": True}
    __table_args__ = (
        UniqueConstraint("provider", "provider_dispute_id", name="uq_dispute_provider_ref"),
    )

    dispute_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    provider: Mapped[str] = mapped_column(Text, nullable=False)  # rail/PSP that raised the dispute
    provider_dispute_id: Mapped[str] = mapped_column(Text, nullable=False)
    payment_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    paylink_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    rail: Mapped[str] = mapped_column(Text, nullable=False)
    merchant_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    org_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    amount_minor: Mapped[int | None] = mapped_column(Numeric(38, 0), nullable=True)
    currency: Mapped[str | None] = mapped_column(Text, nullable=True)
    reason_code: Mapped[str | None] = mapped_column(Text, nullable=True)
    state: Mapped[str] = mapped_column(Text, nullable=False)
    evidence_due_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    submitted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    resolved_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    clawback_requested: Mapped[bool] = mapped_column(
        Boolean, nullable=False, server_default=text("false")
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class DisputeEvidenceRow(Base):
    """Evidence attached to a dispute. Metadata + a JSONB payload only — there is NO object store in
    the repo, so no file bytes are stored (only references / structured fields)."""

    __tablename__ = "dispute_evidence"
    __mapper_args__ = {"eager_defaults": True}

    evidence_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    dispute_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True),
        ForeignKey(f"{Base.metadata.schema}.disputes.dispute_id"),
        nullable=False,
    )
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # receipt|tracking|note|...
    summary: Mapped[str | None] = mapped_column(Text, nullable=True)
    payload: Mapped[dict[str, Any]] = mapped_column(
        JSONB, nullable=False, server_default=text("'{}'")
    )
    external_ref: Mapped[str | None] = mapped_column(
        Text, nullable=True
    )  # opaque pointer (e.g. URL)
    submitted_by: Mapped[str] = mapped_column(Text, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class VerifiedPaylinkRow(Base):
    """Projection of ``chain.paylink.verified`` (settlement truth, A.3) — the authoritative original
    amount used to validate full/partial refunds. Upsert-idempotent (at-least-once delivery)."""

    __tablename__ = "verified_paylinks"
    __mapper_args__ = {"eager_defaults": True}

    paylink_id: Mapped[str] = mapped_column(Text, primary_key=True)
    tx_hash: Mapped[str | None] = mapped_column(Text, nullable=True)
    block_height: Mapped[int | None] = mapped_column(BigInteger, nullable=True)
    amount_minor: Mapped[int | None] = mapped_column(Numeric(38, 0), nullable=True)
    currency: Mapped[str | None] = mapped_column(Text, nullable=True)
    verified_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    payload: Mapped[dict[str, Any]] = mapped_column(
        JSONB, nullable=False, server_default=text("'{}'")
    )


class ProcessedEventRow(Base):
    """Durable consumer-dedupe table (work17 DbDedupe). ``(scope, dedupe_key)`` PK makes a
    redelivery
    a no-op INSERT, so the chain projection applies exactly-once in effect."""

    __tablename__ = "processed_events"

    scope: Mapped[str] = mapped_column(Text, primary_key=True)
    dedupe_key: Mapped[str] = mapped_column(Text, primary_key=True)
    processed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class RefundEventRow(Base):
    """Durable outbox — the source of truth the work15 relay drains onto Kafka.

    ``entity_id`` is the refund_id / dispute_id (per-entity ordering). Payloads carry ids/amount
    metadata only, never secrets.
    """

    __tablename__ = "refund_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name
    entity_id: Mapped[str] = mapped_column(Text, nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    published_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
