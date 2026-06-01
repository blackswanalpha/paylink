"""create compliance schema, kyc_records/risk_scores/flags/activity_events + outbox + indexes

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

SCHEMA = "compliance"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    op.create_table(
        "kyc_records",
        # Opaque ref to identity.users — NO cross-schema FK. PK = one record per user.
        sa.Column("user_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("tier", sa.SmallInteger(), nullable=False, server_default=sa.text("0")),
        sa.Column("provider", sa.Text(), nullable=True),
        # AES-GCM ciphertext of the provider reference — never plaintext.
        sa.Column("provider_ref", sa.Text(), nullable=True),
        sa.Column("documents", postgresql.JSONB(), nullable=True),  # redacted metadata ONLY
        sa.Column("verified_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column("expires_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )

    op.create_table(
        "risk_scores",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("user_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("context", sa.Text(), nullable=False),  # e.g. 'paylink.create'
        sa.Column("score", sa.Numeric(4, 3), nullable=False),  # 0.000 .. 1.000
        sa.Column("decision", sa.Text(), nullable=False),  # allow|block|review
        sa.Column("reasons", postgresql.JSONB(), nullable=False),
        sa.Column(
            "evaluated_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_risk_scores_user_id", "risk_scores", ["user_id"], schema=SCHEMA)

    op.create_table(
        "flags",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("user_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("kind", sa.Text(), nullable=False),  # sanctions|velocity|geo|manual
        sa.Column("severity", sa.Text(), nullable=False),  # info|warn|block
        sa.Column("payload", postgresql.JSONB(), nullable=False),
        sa.Column(
            "raised_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("resolved_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column("resolution", sa.Text(), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_flags_user_id", "flags", ["user_id"], schema=SCHEMA)

    op.create_table(
        "activity_events",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("user_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("action", sa.Text(), nullable=False),
        sa.Column("amount", sa.Numeric(20, 2), nullable=True),
        sa.Column("currency", sa.Text(), nullable=True),
        sa.Column(
            "occurred_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    # Composite index for windowed velocity/AML scans (user_id, occurred_at).
    op.create_index(
        "ix_activity_events_user_time",
        "activity_events",
        ["user_id", "occurred_at"],
        schema=SCHEMA,
    )

    op.create_table(
        "compliance_events",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column("subject_type", sa.Text(), nullable=False),
        sa.Column("subject_id", postgresql.UUID(as_uuid=True), nullable=True),
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
    op.drop_table("compliance_events", schema=SCHEMA)
    op.drop_index("ix_activity_events_user_time", table_name="activity_events", schema=SCHEMA)
    op.drop_table("activity_events", schema=SCHEMA)
    op.drop_index("ix_flags_user_id", table_name="flags", schema=SCHEMA)
    op.drop_table("flags", schema=SCHEMA)
    op.drop_index("ix_risk_scores_user_id", table_name="risk_scores", schema=SCHEMA)
    op.drop_table("risk_scores", schema=SCHEMA)
    op.drop_table("kyc_records", schema=SCHEMA)
