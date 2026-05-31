"""create merchant schema, merchants/bank_accounts/documents/contracts + events + indexes

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

SCHEMA = "merchant"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    op.create_table(
        "merchants",
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), primary_key=True),
        # Opaque ref to identity.organizations — NO cross-schema FK. UNIQUE → one-merchant-per-org
        # (enforces ALREADY_ONBOARDED at the DB level).
        sa.Column("org_id", postgresql.UUID(as_uuid=True), nullable=False, unique=True),
        sa.Column("business_name", sa.Text(), nullable=False),
        sa.Column("registration_no", sa.Text(), nullable=True),
        sa.Column("tax_id", sa.Text(), nullable=True),
        sa.Column("country", sa.Text(), nullable=False),  # ISO 3166-1 alpha-2
        sa.Column("type", sa.Text(), nullable=False),  # individual|company|nonprofit
        sa.Column("status", sa.Text(), nullable=False),  # DRAFT|PENDING_VERIFICATION|ACTIVE|...
        sa.Column("fee_tier", sa.Text(), nullable=False, server_default=sa.text("'standard'")),
        sa.Column("onboarded_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column("suspended_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column("suspended_reason", sa.Text(), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_merchants_status", "merchants", ["status"], schema=SCHEMA)

    op.create_table(
        "bank_accounts",
        sa.Column("bank_account_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "merchant_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.merchants.merchant_id"),
            nullable=False,
        ),
        sa.Column("rail", sa.Text(), nullable=False),  # mpesa|swift|sepa|ach|crypto
        sa.Column("account_ref", sa.Text(), nullable=False),  # KMS-encrypted (AES-GCM ciphertext)
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),  # PENDING_VERIFY|VERIFIED|REVOKED
        sa.Column("verified_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_bank_accounts_merchant_id", "bank_accounts", ["merchant_id"], schema=SCHEMA)

    op.create_table(
        "documents",
        sa.Column("document_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "merchant_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.merchants.merchant_id"),
            nullable=False,
        ),
        sa.Column("kind", sa.Text(), nullable=False),  # cert_incorporation|tax_id|director_id|...
        sa.Column("s3_key", sa.Text(), nullable=False),  # object-store key; bytes live there
        sa.Column(
            "uploaded_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("review", postgresql.JSONB(), nullable=True),  # compliance review result
        schema=SCHEMA,
    )
    op.create_index("ix_documents_merchant_id", "documents", ["merchant_id"], schema=SCHEMA)

    op.create_table(
        "contracts",
        sa.Column("id", sa.BigInteger(), primary_key=True, autoincrement=True),
        sa.Column(
            "merchant_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.merchants.merchant_id"),
            nullable=False,
        ),
        sa.Column("version", sa.Text(), nullable=False),
        # Opaque ref to identity.users — NO cross-schema FK.
        sa.Column("accepted_by", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "accepted_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("ip", postgresql.INET(), nullable=True),
        sa.Column("user_agent", sa.Text(), nullable=True),
        schema=SCHEMA,
    )
    op.create_index("ix_contracts_merchant_id", "contracts", ["merchant_id"], schema=SCHEMA)

    op.create_table(
        "merchant_events",
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
    op.drop_table("merchant_events", schema=SCHEMA)
    op.drop_index("ix_contracts_merchant_id", table_name="contracts", schema=SCHEMA)
    op.drop_table("contracts", schema=SCHEMA)
    op.drop_index("ix_documents_merchant_id", table_name="documents", schema=SCHEMA)
    op.drop_table("documents", schema=SCHEMA)
    op.drop_index("ix_bank_accounts_merchant_id", table_name="bank_accounts", schema=SCHEMA)
    op.drop_table("bank_accounts", schema=SCHEMA)
    op.drop_index("ix_merchants_status", table_name="merchants", schema=SCHEMA)
    op.drop_table("merchants", schema=SCHEMA)
