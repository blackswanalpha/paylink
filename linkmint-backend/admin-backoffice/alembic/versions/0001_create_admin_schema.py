"""create admin schema: staff (Phase 1) + feature_flags/announcements (Phase 2, spec §2.18)

Revision ID: 0001
Revises:
Create Date: 2026-06-01
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "admin"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    # ── Phase 1: default-deny scope grants. `sub` is an opaque ref to identity.users (the JWT
    # `sub`) — NO cross-schema FK. No rows are seeded: a fresh deploy is locked until staff are
    # granted out-of-band. ──
    op.create_table(
        "staff",
        sa.Column("sub", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "scopes",
            postgresql.ARRAY(sa.Text()),
            nullable=False,
            server_default=sa.text("'{}'"),
        ),
        sa.Column("note", sa.Text(), nullable=True),
        sa.Column(
            "updated_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )

    # ── Phase 2 (spec §2.18): created now for fidelity; the read-only console never touches them. ──
    op.create_table(
        "feature_flags",
        sa.Column("flag", sa.Text(), primary_key=True),
        sa.Column("description", sa.Text(), nullable=False),
        sa.Column("enabled", sa.Boolean(), nullable=False, server_default=sa.text("false")),
        sa.Column("rollout_pct", sa.SmallInteger(), nullable=False, server_default=sa.text("0")),
        sa.Column("targeting", postgresql.JSONB(), nullable=True),  # {org_ids, user_ids, tiers}
        sa.Column(
            "updated_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )

    op.create_table(
        "announcements",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("title", sa.Text(), nullable=False),
        sa.Column("body", sa.Text(), nullable=False),
        sa.Column("severity", sa.Text(), nullable=False),  # info|warn|critical
        sa.Column("active_from", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("active_to", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("created_by", postgresql.UUID(as_uuid=True), nullable=False),
        schema=SCHEMA,
    )


def downgrade() -> None:
    op.drop_table("announcements", schema=SCHEMA)
    op.drop_table("feature_flags", schema=SCHEMA)
    op.drop_table("staff", schema=SCHEMA)
