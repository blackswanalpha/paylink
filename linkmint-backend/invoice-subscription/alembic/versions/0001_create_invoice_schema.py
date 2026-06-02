"""create invoice schema, invoices/invoice_lines + outbox + indexes

work19 covers invoices only; the §2.19 `subscriptions` table is deferred to work31 (Phase 3).

Revision ID: 0001
Revises:
Create Date: 2026-06-02
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "invoice"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    op.create_table(
        "invoices",
        sa.Column("invoice_id", postgresql.UUID(as_uuid=True), primary_key=True),
        # Opaque refs to identity — NO cross-schema FK.
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("customer_id", postgresql.UUID(as_uuid=True), nullable=True),
        # Payee chain address (0x-prefixed 20-byte hex) — becomes the backing PayLink receiver.
        sa.Column("payee_addr", sa.Text(), nullable=False),
        # Backing PayLink id, set on finalize. UNIQUE so a chain event maps to exactly one invoice.
        sa.Column("pl_id", sa.Text(), nullable=True),
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("subtotal", sa.Numeric(38, 0), nullable=False),
        sa.Column("tax", sa.Numeric(38, 0), nullable=False, server_default=sa.text("0")),
        sa.Column("total", sa.Numeric(38, 0), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),  # DRAFT|OPEN|PAID|VOID|OVERDUE
        sa.Column("due_at", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("paid_at", sa.TIMESTAMP(timezone=True), nullable=True),
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
    op.create_index("ix_invoices_merchant_id", "invoices", ["merchant_id"], schema=SCHEMA)
    # Unique index (Postgres treats NULLs as distinct, so unfinalized invoices coexist freely).
    op.create_index("uq_invoices_pl_id", "invoices", ["pl_id"], unique=True, schema=SCHEMA)
    # Composite index for the overdue sweeper scan (status, due_at).
    op.create_index("ix_invoices_status_due", "invoices", ["status", "due_at"], schema=SCHEMA)

    op.create_table(
        "invoice_lines",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column(
            "invoice_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.invoices.invoice_id"),
            nullable=False,
        ),
        sa.Column("description", sa.Text(), nullable=False),
        sa.Column("quantity", sa.Numeric(20, 4), nullable=False),
        sa.Column("unit_price", sa.Numeric(38, 0), nullable=False),
        sa.Column("total", sa.Numeric(38, 0), nullable=False),
        sa.Column("tax_rate", sa.Numeric(5, 4), nullable=False, server_default=sa.text("0")),
        schema=SCHEMA,
    )
    op.create_index("ix_invoice_lines_invoice_id", "invoice_lines", ["invoice_id"], schema=SCHEMA)

    op.create_table(
        "invoice_events",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("kind", sa.Text(), nullable=False),  # logical event name (invoice.*)
        sa.Column("invoice_id", postgresql.UUID(as_uuid=True), nullable=False),
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
    # Partial index over the outbox drain hot path (unpublished rows, ordered by id).
    op.create_index(
        "ix_invoice_events_unpublished",
        "invoice_events",
        ["id"],
        schema=SCHEMA,
        postgresql_where=sa.text("published_at IS NULL"),
    )


def downgrade() -> None:
    op.drop_index("ix_invoice_events_unpublished", table_name="invoice_events", schema=SCHEMA)
    op.drop_table("invoice_events", schema=SCHEMA)
    op.drop_index("ix_invoice_lines_invoice_id", table_name="invoice_lines", schema=SCHEMA)
    op.drop_table("invoice_lines", schema=SCHEMA)
    op.drop_index("ix_invoices_status_due", table_name="invoices", schema=SCHEMA)
    op.drop_index("uq_invoices_pl_id", table_name="invoices", schema=SCHEMA)
    op.drop_index("ix_invoices_merchant_id", table_name="invoices", schema=SCHEMA)
    op.drop_table("invoices", schema=SCHEMA)
