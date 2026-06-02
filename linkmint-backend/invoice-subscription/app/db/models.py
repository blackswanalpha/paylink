"""ORM models for the ``invoice`` schema (mirrors backendfeatures.md §2.19 DDL, invoices subset).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.

There are NO cross-schema FKs: ``merchant_id`` / ``customer_id`` are OPAQUE refs to identity, and
``pl_id`` is an opaque ref to the paylink-service PayLink. Money is stored as integer minor units in
``NUMERIC(38,0)``. The ``subscriptions`` table from §2.19 is deferred to work31 (Phase 3).
"""

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import BigInteger, DateTime, ForeignKey, Numeric, Text, text
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class InvoiceRow(Base):
    """One invoice — aggregates its lines into a single backing PayLink (``pl_id``) on finalize."""

    __tablename__ = "invoices"
    __mapper_args__ = {"eager_defaults": True}

    invoice_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    # Opaque refs to identity — NO cross-schema FK.
    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    customer_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    # The payee chain address (0x-prefixed 20-byte hex) — becomes the PayLink ``receiver``.
    payee_addr: Mapped[str] = mapped_column(Text, nullable=False)
    # The backing PayLink id — set on finalize (UNIQUE so a chain event maps to one row).
    pl_id: Mapped[str | None] = mapped_column(Text, nullable=True, unique=True)
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    subtotal: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    tax: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False, server_default=text("0"))
    total: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # DRAFT|OPEN|PAID|VOID|OVERDUE
    due_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    paid_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class InvoiceLineRow(Base):
    """A single invoice line. ``total`` is the net line amount (quantity × unit_price), pre-tax."""

    __tablename__ = "invoice_lines"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    invoice_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("invoices.invoice_id"), nullable=False
    )
    description: Mapped[str] = mapped_column(Text, nullable=False)
    quantity: Mapped[Decimal] = mapped_column(Numeric(20, 4), nullable=False)
    unit_price: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)  # minor units
    total: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)  # minor units, pre-tax
    tax_rate: Mapped[Decimal] = mapped_column(
        Numeric(5, 4), nullable=False, server_default=text("0")
    )


class InvoiceEventRow(Base):
    """Durable outbox — the source of truth the relay (work15) drains onto Kafka.

    Payloads carry ids/amount metadata only, never secrets.
    """

    __tablename__ = "invoice_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name (invoice.*)
    invoice_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    published_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
