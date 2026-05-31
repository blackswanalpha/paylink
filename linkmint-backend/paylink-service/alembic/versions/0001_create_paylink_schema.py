"""create paylink schema, paylinks + paylink_events tables and indexes

Revision ID: 0001
Revises:
Create Date: 2026-05-29
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "paylink"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    op.create_table(
        "paylinks",
        sa.Column("pl_id", sa.Text(), primary_key=True),  # chain-issued hash, 0x-hex
        sa.Column("creator_addr", sa.Text(), nullable=False),
        sa.Column("receiver_addr", sa.Text(), nullable=False),
        sa.Column("owner_addr", sa.Text(), nullable=False),
        sa.Column("amount", sa.Numeric(38, 0), nullable=False),  # minor units, decimal-safe
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),
        sa.Column("expiry", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("usage", sa.Text(), nullable=False),  # 'single' | 'multi'
        sa.Column("metadata", postgresql.JSONB(), nullable=True),
        sa.Column("rules", postgresql.JSONB(), nullable=True),  # immutable after creation
        sa.Column("chain_tx_hash", sa.Text(), nullable=True),
        sa.Column("vote_count", sa.Integer(), nullable=False, server_default=sa.text("0")),
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
        sa.Column("verified_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    op.create_index(
        "paylinks_creator_idx",
        "paylinks",
        ["creator_addr", "status", sa.text("created_at DESC")],
        schema=SCHEMA,
    )
    op.create_index(
        "paylinks_receiver_idx",
        "paylinks",
        ["receiver_addr", "status", sa.text("created_at DESC")],
        schema=SCHEMA,
    )
    op.create_index(
        "paylinks_expiry_idx",
        "paylinks",
        ["expiry"],
        schema=SCHEMA,
        postgresql_where=sa.text("status = 'PENDING'"),
    )

    op.create_table(
        "paylink_events",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column(
            "pl_id",
            sa.Text(),
            sa.ForeignKey(f"{SCHEMA}.paylinks.pl_id"),
            nullable=False,
        ),
        sa.Column("kind", sa.Text(), nullable=False),
        sa.Column("payload", postgresql.JSONB(), nullable=False),
        sa.Column(
            "occurred_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )


def downgrade() -> None:
    op.drop_table("paylink_events", schema=SCHEMA)
    op.drop_index("paylinks_expiry_idx", table_name="paylinks", schema=SCHEMA)
    op.drop_index("paylinks_receiver_idx", table_name="paylinks", schema=SCHEMA)
    op.drop_index("paylinks_creator_idx", table_name="paylinks", schema=SCHEMA)
    op.drop_table("paylinks", schema=SCHEMA)
