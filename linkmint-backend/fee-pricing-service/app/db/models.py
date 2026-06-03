"""ORM models for the ``pricing`` schema (work21 — the work item body is the authoritative scope;
``backendfeatures.md`` §2.8 is not in the working tree).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.

There are NO cross-schema FKs: ``merchant_id`` / ``org_id`` are OPAQUE refs to merchant-onboarding /
identity. Money is stored as integer minor units in ``NUMERIC(38,0)``; FX multipliers (not money)
use ``NUMERIC(38,18)``. The platform fee modelled here is DISTINCT from the on-chain 0.5% PLN
inflation fee (rules.md A.5) — this service never touches chain-fee logic.
"""

import uuid
from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import BigInteger, Boolean, DateTime, Integer, Numeric, Text, UniqueConstraint, text
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class TierRow(Base):
    """A fee tier and its platform-fee schedule (the superset of work21 + work10 tier names)."""

    __tablename__ = "tiers"
    __mapper_args__ = {"eager_defaults": True}

    tier: Mapped[str] = mapped_column(Text, primary_key=True)
    display_name: Mapped[str] = mapped_column(Text, nullable=False)
    # basis points, e.g. 250 = 2.5%
    platform_pct_bps: Mapped[int] = mapped_column(Integer, nullable=False)
    platform_fixed: Mapped[Decimal] = mapped_column(
        Numeric(38, 0), nullable=False, server_default=text("0")
    )  # flat per-tx component, minor units
    fixed_currency: Mapped[str] = mapped_column(Text, nullable=False, server_default=text("'KES'"))
    active: Mapped[bool] = mapped_column(Boolean, nullable=False, server_default=text("true"))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class RailFeeRow(Base):
    """Per-rail fee (A.4 rails). ``tier`` NULL = global; a (rail, tier) row overrides the global."""

    __tablename__ = "rail_fee_schedules"
    __mapper_args__ = {"eager_defaults": True}
    __table_args__ = (UniqueConstraint("rail", "tier", name="uq_rail_tier"),)

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    rail: Mapped[str] = mapped_column(Text, nullable=False)  # mpesa|card|bank|crypto
    tier: Mapped[str | None] = mapped_column(Text, nullable=True)  # NULL = applies to all tiers
    pct_bps: Mapped[int] = mapped_column(Integer, nullable=False)
    fixed: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False, server_default=text("0"))
    fixed_currency: Mapped[str] = mapped_column(Text, nullable=False, server_default=text("'KES'"))
    active: Mapped[bool] = mapped_column(Boolean, nullable=False, server_default=text("true"))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class MerchantPricingRow(Base):
    """A merchant's current tier (driven by the merchant-event consumer; one row per merchant)."""

    __tablename__ = "merchant_pricing"
    __mapper_args__ = {"eager_defaults": True}

    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    # Captured from merchant.onboarded (carries org_id); the org-membership read gate uses it.
    org_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    tier: Mapped[str] = mapped_column(Text, nullable=False)
    effective_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    source: Mapped[str] = mapped_column(Text, nullable=False)  # onboarded|fee_tier.changed|manual
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class FxRateRow(Base):
    """Durable audit trail of every rate we fetched (Redis is the short-lived hot cache)."""

    __tablename__ = "fx_rates"
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    base_currency: Mapped[str] = mapped_column(Text, nullable=False)
    quote_currency: Mapped[str] = mapped_column(Text, nullable=False)
    rate: Mapped[Decimal] = mapped_column(Numeric(38, 18), nullable=False)  # 1 base = <rate> quote
    source: Mapped[str] = mapped_column(Text, nullable=False)  # static|http:<provider>|fallback
    fetched_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class QuoteRow(Base):
    """Every issued quote, with the LOCKED fx rate + full breakdown stored for audit."""

    __tablename__ = "quotes"
    __mapper_args__ = {"eager_defaults": True}

    quote_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    tier: Mapped[str] = mapped_column(Text, nullable=False)
    rail: Mapped[str] = mapped_column(Text, nullable=False)
    # requested amount, minor units, in `currency`
    gross: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    settle_currency: Mapped[str] = mapped_column(Text, nullable=False)
    platform_fee: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    rail_fee: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    net: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    # FX columns set only when a currency conversion applied; fx_rate is the LOCKED rate (audit).
    fx_base: Mapped[str | None] = mapped_column(Text, nullable=True)
    fx_quote: Mapped[str | None] = mapped_column(Text, nullable=True)
    fx_rate: Mapped[Decimal | None] = mapped_column(Numeric(38, 18), nullable=True)
    breakdown: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class PlatformFeeAccrualRow(Base):
    """A realized platform fee awaiting monthly aggregation into an invoice."""

    __tablename__ = "platform_fee_accruals"
    __mapper_args__ = {"eager_defaults": True}
    __table_args__ = (UniqueConstraint("merchant_id", "source_ref", name="uq_accrual_source"),)

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    period: Mapped[str] = mapped_column(Text, nullable=False)  # 'YYYY-MM' (UTC), from occurred_at
    # platform fee, minor units
    amount: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    source_ref: Mapped[str] = mapped_column(Text, nullable=False)  # payment_id / pl_id / quote_id
    quote_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    # NULL = unbilled; set when rolled into an invoice
    invoice_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class PlatformFeeInvoiceRow(Base):
    """One platform-fee invoice per merchant per period (idempotent generation)."""

    __tablename__ = "platform_fee_invoices"
    __mapper_args__ = {"eager_defaults": True}
    __table_args__ = (UniqueConstraint("merchant_id", "period", name="uq_invoice_merchant_period"),)

    invoice_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    merchant_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    period: Mapped[str] = mapped_column(Text, nullable=False)  # 'YYYY-MM'
    currency: Mapped[str] = mapped_column(Text, nullable=False)
    total_fee: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    line_count: Mapped[int] = mapped_column(Integer, nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # ISSUED
    issued_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class PricingEventRow(Base):
    """Durable outbox — the source of truth the relay (work15) drains onto Kafka.

    ``entity_id`` is TEXT (fx.rate.updated keys on a currency pair, not a UUID). Payloads carry
    ids/amount metadata only, never secrets.
    """

    __tablename__ = "pricing_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name
    entity_id: Mapped[str] = mapped_column(Text, nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    published_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
