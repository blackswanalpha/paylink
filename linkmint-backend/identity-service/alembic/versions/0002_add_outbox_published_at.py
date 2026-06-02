"""add published_at to identity_events (work15 outbox-drain relay)

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

SCHEMA = "identity"


def upgrade() -> None:
    op.add_column(
        "identity_events",
        sa.Column("published_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    op.create_index(
        "identity_events_unpublished_idx",
        "identity_events",
        ["id"],
        unique=False,
        schema=SCHEMA,
        postgresql_where=sa.text("published_at IS NULL"),
    )


def downgrade() -> None:
    op.drop_index("identity_events_unpublished_idx", table_name="identity_events", schema=SCHEMA)
    op.drop_column("identity_events", "published_at", schema=SCHEMA)
