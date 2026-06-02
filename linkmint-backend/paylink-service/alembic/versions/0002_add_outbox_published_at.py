"""add published_at to paylink_events (work15 outbox-drain relay)

Revision ID: 0002
Revises: 0001
Create Date: 2026-06-02
"""

from collections.abc import Sequence

import sqlalchemy as sa

from alembic import op

revision: str = "0002"
down_revision: str | None = "0001"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "paylink"


def upgrade() -> None:
    # Nullable: existing rows stay NULL (= unpublished); the relay drains WHERE published_at IS NULL.
    op.add_column(
        "paylink_events",
        sa.Column("published_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    # Partial index keeps the relay's "next unpublished batch" scan cheap as the table grows.
    op.create_index(
        "paylink_events_unpublished_idx",
        "paylink_events",
        ["id"],
        unique=False,
        schema=SCHEMA,
        postgresql_where=sa.text("published_at IS NULL"),
    )


def downgrade() -> None:
    op.drop_index("paylink_events_unpublished_idx", table_name="paylink_events", schema=SCHEMA)
    op.drop_column("paylink_events", "published_at", schema=SCHEMA)
