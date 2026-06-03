"""create pricing schema — tiers, rail fee schedules, merchant pricing, fx rates, quotes,
platform-fee accruals/invoices + outbox, with seed tiers and global rail schedules.

work21 — the work item body is the authoritative scope (backendfeatures.md §2.8 is not in the tree).

Revision ID: 0001
Revises:
Create Date: 2026-06-03
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "pricing"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    # 1. tiers — the platform-fee schedule per tier (the superset; work21 + work10 names).
    op.create_table(
        "tiers",
        sa.Column("tier", sa.Text(), primary_key=True),
        sa.Column("display_name", sa.Text(), nullable=False),
        sa.Column("platform_pct_bps", sa.Integer(), nullable=False),  # basis points (250 = 2.5%)
        sa.Column("platform_fixed", sa.Numeric(38, 0), nullable=False, server_default=sa.text("0")),
        sa.Column("fixed_currency", sa.Text(), nullable=False, server_default=sa.text("'KES'")),
        sa.Column("active", sa.Boolean(), nullable=False, server_default=sa.text("true")),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column(
            "updated_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )

    # 2. rail_fee_schedules — per-rail fee (A.4 rail set). tier NULL = global; (rail, tier) overrides.
    op.create_table(
        "rail_fee_schedules",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("rail", sa.Text(), nullable=False),  # mpesa|card|bank|crypto
        sa.Column("tier", sa.Text(), nullable=True),  # NULL = applies to all tiers
        sa.Column("pct_bps", sa.Integer(), nullable=False),
        sa.Column("fixed", sa.Numeric(38, 0), nullable=False, server_default=sa.text("0")),
        sa.Column("fixed_currency", sa.Text(), nullable=False, server_default=sa.text("'KES'")),
        sa.Column("active", sa.Boolean(), nullable=False, server_default=sa.text("true")),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.UniqueConstraint("rail", "tier", name="uq_rail_tier"),
        schema=SCHEMA,
    )
    op.create_index("ix_rail_fee_rail", "rail_fee_schedules", ["rail"], schema=SCHEMA)

    # 3. merchant_pricing — a merchant's current tier (consumer-driven; one row per merchant).
    op.create_table(
        "merchant_pricing",
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("org_id", postgresql.UUID(as_uuid=True), nullable=True),  # opaque ref, NO FK
        sa.Column("tier", sa.Text(), nullable=False),
        sa.Column(
            "effective_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("source", sa.Text(), nullable=False),  # onboarded|fee_tier.changed|manual
        sa.Column(
            "updated_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_merchant_pricing_org", "merchant_pricing", ["org_id"], schema=SCHEMA)

    # 4. fx_rates — durable audit trail of every rate fetched (Redis is the short-lived hot cache).
    op.create_table(
        "fx_rates",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("base_currency", sa.Text(), nullable=False),
        sa.Column("quote_currency", sa.Text(), nullable=False),
        sa.Column("rate", sa.Numeric(38, 18), nullable=False),  # 1 base = <rate> quote (mid-market)
        sa.Column("source", sa.Text(), nullable=False),  # static|http:<provider>|fallback
        sa.Column(
            "fetched_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index(
        "ix_fx_pair_time",
        "fx_rates",
        ["base_currency", "quote_currency", sa.text("fetched_at DESC")],
        schema=SCHEMA,
    )

    # 5. quotes — every issued quote, with the LOCKED fx rate + full breakdown stored for audit.
    op.create_table(
        "quotes",
        sa.Column("quote_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("tier", sa.Text(), nullable=False),
        sa.Column("rail", sa.Text(), nullable=False),
        sa.Column("gross", sa.Numeric(38, 0), nullable=False),
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("settle_currency", sa.Text(), nullable=False),
        sa.Column("platform_fee", sa.Numeric(38, 0), nullable=False),
        sa.Column("rail_fee", sa.Numeric(38, 0), nullable=False),
        sa.Column("net", sa.Numeric(38, 0), nullable=False),
        sa.Column("fx_base", sa.Text(), nullable=True),
        sa.Column("fx_quote", sa.Text(), nullable=True),
        sa.Column("fx_rate", sa.Numeric(38, 18), nullable=True),  # the LOCKED rate used (audit)
        sa.Column("breakdown", postgresql.JSONB(), nullable=False),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_quotes_merchant", "quotes", ["merchant_id"], schema=SCHEMA)

    # 6. platform_fee_accruals — realized platform fees awaiting monthly aggregation.
    op.create_table(
        "platform_fee_accruals",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("period", sa.Text(), nullable=False),  # 'YYYY-MM' (UTC), derived from occurred_at
        sa.Column("amount", sa.Numeric(38, 0), nullable=False),
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("source_ref", sa.Text(), nullable=False),  # payment_id / pl_id / quote_id
        sa.Column("quote_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("invoice_id", postgresql.UUID(as_uuid=True), nullable=True),  # NULL = unbilled
        sa.Column("occurred_at", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.UniqueConstraint("merchant_id", "source_ref", name="uq_accrual_source"),
        schema=SCHEMA,
    )
    # Partial index over the monthly-aggregation hot path (unbilled accruals for a merchant+period).
    op.create_index(
        "ix_accrual_unbilled",
        "platform_fee_accruals",
        ["merchant_id", "period"],
        schema=SCHEMA,
        postgresql_where=sa.text("invoice_id IS NULL"),
    )

    # 7. platform_fee_invoices — one invoice per merchant per period (idempotent generation).
    op.create_table(
        "platform_fee_invoices",
        sa.Column("invoice_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("period", sa.Text(), nullable=False),  # 'YYYY-MM'
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("total_fee", sa.Numeric(38, 0), nullable=False),
        sa.Column("line_count", sa.Integer(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),  # ISSUED
        sa.Column(
            "issued_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.UniqueConstraint("merchant_id", "period", name="uq_invoice_merchant_period"),
        schema=SCHEMA,
    )
    op.create_index("ix_pfi_merchant", "platform_fee_invoices", ["merchant_id"], schema=SCHEMA)

    # 8. pricing_events — the durable outbox the work15 relay drains (entity_id is TEXT).
    op.create_table(
        "pricing_events",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("kind", sa.Text(), nullable=False),  # logical event name
        sa.Column("entity_id", sa.Text(), nullable=False),
        sa.Column("payload", postgresql.JSONB(), nullable=False),
        sa.Column("published_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index(
        "ix_pricing_events_unpublished",
        "pricing_events",
        ["id"],
        schema=SCHEMA,
        postgresql_where=sa.text("published_at IS NULL"),
    )

    # ── Seed data: the 5 tiers + 4 global rail schedules so a fresh DB can quote immediately.
    # These are dev defaults the platform can tune via the admin tier surface later.
    tiers_tbl = sa.table(
        "tiers",
        sa.column("tier", sa.Text),
        sa.column("display_name", sa.Text),
        sa.column("platform_pct_bps", sa.Integer),
        sa.column("platform_fixed", sa.Numeric(38, 0)),
        sa.column("fixed_currency", sa.Text),
        schema=SCHEMA,
    )
    op.bulk_insert(
        tiers_tbl,
        [
            {
                "tier": "standard",
                "display_name": "Standard",
                "platform_pct_bps": 250,
                "platform_fixed": 0,
                "fixed_currency": "KES",
            },
            {
                "tier": "startup",
                "display_name": "Startup",
                "platform_pct_bps": 150,
                "platform_fixed": 0,
                "fixed_currency": "KES",
            },
            {
                "tier": "growth",
                "display_name": "Growth",
                "platform_pct_bps": 200,
                "platform_fixed": 0,
                "fixed_currency": "KES",
            },
            {
                "tier": "scale",
                "display_name": "Scale",
                "platform_pct_bps": 120,
                "platform_fixed": 0,
                "fixed_currency": "KES",
            },
            {
                "tier": "enterprise",
                "display_name": "Enterprise",
                "platform_pct_bps": 80,
                "platform_fixed": 0,
                "fixed_currency": "KES",
            },
        ],
    )

    rail_tbl = sa.table(
        "rail_fee_schedules",
        sa.column("rail", sa.Text),
        sa.column("tier", sa.Text),
        sa.column("pct_bps", sa.Integer),
        sa.column("fixed", sa.Numeric(38, 0)),
        sa.column("fixed_currency", sa.Text),
        schema=SCHEMA,
    )
    op.bulk_insert(
        rail_tbl,
        [
            {"rail": "mpesa", "tier": None, "pct_bps": 150, "fixed": 0, "fixed_currency": "KES"},
            {"rail": "card", "tier": None, "pct_bps": 290, "fixed": 30, "fixed_currency": "KES"},
            {"rail": "bank", "tier": None, "pct_bps": 50, "fixed": 0, "fixed_currency": "KES"},
            {"rail": "crypto", "tier": None, "pct_bps": 100, "fixed": 0, "fixed_currency": "KES"},
        ],
    )


def downgrade() -> None:
    op.drop_index("ix_pricing_events_unpublished", table_name="pricing_events", schema=SCHEMA)
    op.drop_table("pricing_events", schema=SCHEMA)
    op.drop_index("ix_pfi_merchant", table_name="platform_fee_invoices", schema=SCHEMA)
    op.drop_table("platform_fee_invoices", schema=SCHEMA)
    op.drop_index("ix_accrual_unbilled", table_name="platform_fee_accruals", schema=SCHEMA)
    op.drop_table("platform_fee_accruals", schema=SCHEMA)
    op.drop_index("ix_quotes_merchant", table_name="quotes", schema=SCHEMA)
    op.drop_table("quotes", schema=SCHEMA)
    op.drop_index("ix_fx_pair_time", table_name="fx_rates", schema=SCHEMA)
    op.drop_table("fx_rates", schema=SCHEMA)
    op.drop_index("ix_merchant_pricing_org", table_name="merchant_pricing", schema=SCHEMA)
    op.drop_table("merchant_pricing", schema=SCHEMA)
    op.drop_index("ix_rail_fee_rail", table_name="rail_fee_schedules", schema=SCHEMA)
    op.drop_table("rail_fee_schedules", schema=SCHEMA)
    op.drop_table("tiers", schema=SCHEMA)
