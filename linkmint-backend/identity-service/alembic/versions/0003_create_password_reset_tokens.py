"""create password_reset_tokens (single-use, hashed-at-rest reset tokens)

Revision ID: 0003
Revises: 0002
Create Date: 2026-06-03
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0003"
down_revision: str | None = "0002"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "identity"


def upgrade() -> None:
    op.create_table(
        "password_reset_tokens",
        sa.Column("token_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "user_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            nullable=False,
        ),
        sa.Column("token_hash", sa.Text(), nullable=False),  # SHA-256 hash of the token
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("expires_at", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("used_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    # Unique so a presented token resolves to at most one row (lookup-by-hash == constant-time match).
    op.create_index(
        "uq_password_reset_token_hash",
        "password_reset_tokens",
        ["token_hash"],
        schema=SCHEMA,
        unique=True,
    )
    op.create_index(
        "ix_password_reset_user_id", "password_reset_tokens", ["user_id"], schema=SCHEMA
    )


def downgrade() -> None:
    op.drop_index("ix_password_reset_user_id", table_name="password_reset_tokens", schema=SCHEMA)
    op.drop_index("uq_password_reset_token_hash", table_name="password_reset_tokens", schema=SCHEMA)
    op.drop_table("password_reset_tokens", schema=SCHEMA)
