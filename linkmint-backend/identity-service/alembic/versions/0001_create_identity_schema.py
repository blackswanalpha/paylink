"""create identity schema, users/orgs/memberships/api_keys/sessions/mfa/oauth + events + indexes

Revision ID: 0001
Revises:
Create Date: 2026-05-31
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "identity"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    op.create_table(
        "users",
        sa.Column("user_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("email", sa.Text(), nullable=True),
        sa.Column("phone", sa.Text(), nullable=True),
        sa.Column("password_hash", sa.Text(), nullable=True),  # argon2id; null for OAuth-only
        sa.Column("kyc_tier", sa.SmallInteger(), nullable=False, server_default=sa.text("0")),
        sa.Column("status", sa.Text(), nullable=False),  # ACTIVE|SUSPENDED|DELETED
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("last_login_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    # Partial-unique: many NULL emails/phones may coexist (OAuth-only / phone-only users).
    op.create_index(
        "uq_users_email",
        "users",
        ["email"],
        schema=SCHEMA,
        unique=True,
        postgresql_where=sa.text("email IS NOT NULL"),
    )
    op.create_index(
        "uq_users_phone",
        "users",
        ["phone"],
        schema=SCHEMA,
        unique=True,
        postgresql_where=sa.text("phone IS NOT NULL"),
    )

    op.create_table(
        "oauth_identities",
        sa.Column("provider", sa.Text(), primary_key=True),  # google|apple|github
        sa.Column("subject", sa.Text(), primary_key=True),
        sa.Column(
            "user_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            nullable=False,
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_oauth_identities_user_id", "oauth_identities", ["user_id"], schema=SCHEMA)

    op.create_table(
        "mfa_factors",
        sa.Column(
            "user_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            primary_key=True,
        ),
        sa.Column("kind", sa.Text(), primary_key=True),  # totp|webauthn|sms_otp
        sa.Column("secret", sa.Text(), nullable=False),  # AES-GCM ciphertext (KMS stand-in)
        sa.Column(
            "enrolled_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("activated_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )

    op.create_table(
        "organizations",
        sa.Column("org_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("type", sa.Text(), nullable=False),  # merchant|developer|admin
        sa.Column(
            "created_by",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            nullable=False,
        ),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_organizations_created_by", "organizations", ["created_by"], schema=SCHEMA)

    op.create_table(
        "memberships",
        sa.Column(
            "org_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.organizations.org_id"),
            primary_key=True,
        ),
        sa.Column(
            "user_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            primary_key=True,
        ),
        sa.Column("role", sa.Text(), nullable=False),
        schema=SCHEMA,
    )
    op.create_index("ix_memberships_user_id", "memberships", ["user_id"], schema=SCHEMA)

    op.create_table(
        "api_keys",
        sa.Column("api_key_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "org_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.organizations.org_id"),
            nullable=False,
        ),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("prefix", sa.Text(), nullable=False),  # displayed; e.g. lm_live_AbCd1234
        sa.Column("hash", sa.Text(), nullable=False),  # argon2id of the full key
        sa.Column("scopes", postgresql.ARRAY(sa.Text()), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),  # ACTIVE|REVOKED
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("revoked_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_api_keys_org_status", "api_keys", ["org_id", "status"], schema=SCHEMA)
    op.create_index("ix_api_keys_prefix", "api_keys", ["prefix"], schema=SCHEMA)

    op.create_table(
        "sessions",
        sa.Column("session_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "user_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.users.user_id"),
            nullable=False,
        ),
        sa.Column("refresh_token", sa.Text(), nullable=False),  # SHA-256 hash of the token
        sa.Column("user_agent", sa.Text(), nullable=True),
        sa.Column("ip", postgresql.INET(), nullable=True),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("expires_at", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("revoked_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_sessions_user_id", "sessions", ["user_id"], schema=SCHEMA)
    op.create_index(
        "uq_sessions_refresh_token", "sessions", ["refresh_token"], schema=SCHEMA, unique=True
    )

    op.create_table(
        "identity_events",
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
    op.drop_table("identity_events", schema=SCHEMA)
    op.drop_index("uq_sessions_refresh_token", table_name="sessions", schema=SCHEMA)
    op.drop_index("ix_sessions_user_id", table_name="sessions", schema=SCHEMA)
    op.drop_table("sessions", schema=SCHEMA)
    op.drop_index("ix_api_keys_prefix", table_name="api_keys", schema=SCHEMA)
    op.drop_index("ix_api_keys_org_status", table_name="api_keys", schema=SCHEMA)
    op.drop_table("api_keys", schema=SCHEMA)
    op.drop_index("ix_memberships_user_id", table_name="memberships", schema=SCHEMA)
    op.drop_table("memberships", schema=SCHEMA)
    op.drop_index("ix_organizations_created_by", table_name="organizations", schema=SCHEMA)
    op.drop_table("organizations", schema=SCHEMA)
    op.drop_table("mfa_factors", schema=SCHEMA)
    op.drop_index("ix_oauth_identities_user_id", table_name="oauth_identities", schema=SCHEMA)
    op.drop_table("oauth_identities", schema=SCHEMA)
    op.drop_index("uq_users_phone", table_name="users", schema=SCHEMA)
    op.drop_index("uq_users_email", table_name="users", schema=SCHEMA)
    op.drop_table("users", schema=SCHEMA)
