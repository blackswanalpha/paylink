"""create refund schema — refunds, disputes, dispute_evidence, verified_paylinks projection,
processed_events (DbDedupe), and the refund_events outbox.

work22 — the work item body is the authoritative scope (backendfeatures.md §2.9 is not in the tree).
Non-custodial (A.1): this service records state + emits instructions; it never holds funds.

Revision ID: 0001
Revises:
Create Date: 2026-06-13
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "refund"

_REFUND_STATES = "('REQUESTED','APPROVED','REJECTED','PROCESSING','COMPLETED','FAILED')"
_DISPUTE_STATES = "('OPEN','SUBMITTED','WON','LOST','EXPIRED')"


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    # 1. refunds — sender/merchant-initiated refund lifecycle.
    op.create_table(
        "refunds",
        sa.Column("refund_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("payment_id", sa.Text(), nullable=False),  # opaque (payment-orchestrator)
        sa.Column("paylink_id", sa.Text(), nullable=False),  # opaque (chain PayLink id)
        sa.Column("rail", sa.Text(), nullable=False),  # mpesa|card|bank|crypto (opaque, A.4)
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=True),  # opaque ref, NO FK
        sa.Column("org_id", postgresql.UUID(as_uuid=True), nullable=True),  # opaque ref, NO FK
        sa.Column("requested_by", sa.Text(), nullable=False),
        sa.Column("amount_minor", sa.Numeric(38, 0), nullable=False),
        sa.Column("currency", sa.Text(), nullable=False),
        sa.Column("reason", sa.Text(), nullable=True),
        sa.Column("state", sa.Text(), nullable=False),
        sa.Column("is_partial", sa.Boolean(), nullable=False),
        sa.Column("approved_by", sa.Text(), nullable=True),
        sa.Column("failure_reason", sa.Text(), nullable=True),
        sa.Column("reversal_ref", sa.Text(), nullable=True),
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
        sa.CheckConstraint("amount_minor > 0", name="ck_refund_amount_positive"),
        sa.CheckConstraint(f"state IN {_REFUND_STATES}", name="ck_refund_state"),
        schema=SCHEMA,
    )
    op.create_index("ix_refunds_payment", "refunds", ["payment_id"], schema=SCHEMA)
    op.create_index("ix_refunds_state", "refunds", ["state"], schema=SCHEMA)
    op.create_index("ix_refunds_org", "refunds", ["org_id"], schema=SCHEMA)

    # 2. disputes — rail-initiated dispute/chargeback lifecycle. UNIQUE(provider, provider_dispute_id)
    # is the webhook anti-replay key (A.7).
    op.create_table(
        "disputes",
        sa.Column("dispute_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("provider", sa.Text(), nullable=False),
        sa.Column("provider_dispute_id", sa.Text(), nullable=False),
        sa.Column("payment_id", sa.Text(), nullable=True),
        sa.Column("paylink_id", sa.Text(), nullable=True),
        sa.Column("rail", sa.Text(), nullable=False),
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("org_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("amount_minor", sa.Numeric(38, 0), nullable=True),
        sa.Column("currency", sa.Text(), nullable=True),
        sa.Column("reason_code", sa.Text(), nullable=True),
        sa.Column("state", sa.Text(), nullable=False),
        sa.Column("evidence_due_at", sa.TIMESTAMP(timezone=True), nullable=False),
        sa.Column("submitted_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column("resolved_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column(
            "clawback_requested", sa.Boolean(), nullable=False, server_default=sa.text("false")
        ),
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
        sa.CheckConstraint(f"state IN {_DISPUTE_STATES}", name="ck_dispute_state"),
        sa.UniqueConstraint("provider", "provider_dispute_id", name="uq_dispute_provider_ref"),
        schema=SCHEMA,
    )
    # Sweeper hot path: OPEN disputes ordered by their evidence deadline.
    op.create_index("ix_disputes_due", "disputes", ["state", "evidence_due_at"], schema=SCHEMA)
    op.create_index("ix_disputes_payment", "disputes", ["payment_id"], schema=SCHEMA)

    # 3. dispute_evidence — metadata + JSONB payload only (no object store in the repo).
    op.create_table(
        "dispute_evidence",
        sa.Column("evidence_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "dispute_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.disputes.dispute_id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("kind", sa.Text(), nullable=False),
        sa.Column("summary", sa.Text(), nullable=True),
        sa.Column("payload", postgresql.JSONB(), nullable=False, server_default=sa.text("'{}'")),
        sa.Column("external_ref", sa.Text(), nullable=True),
        sa.Column("submitted_by", sa.Text(), nullable=False),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )
    op.create_index("ix_evidence_dispute", "dispute_evidence", ["dispute_id"], schema=SCHEMA)

    # 4. verified_paylinks — projection of chain.paylink.verified (settlement truth, A.3): the
    # authoritative original amount used to validate full/partial refunds.
    op.create_table(
        "verified_paylinks",
        sa.Column("paylink_id", sa.Text(), primary_key=True),
        sa.Column("tx_hash", sa.Text(), nullable=True),
        sa.Column("block_height", sa.BigInteger(), nullable=True),
        sa.Column("amount_minor", sa.Numeric(38, 0), nullable=True),
        sa.Column("currency", sa.Text(), nullable=True),
        sa.Column(
            "verified_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("payload", postgresql.JSONB(), nullable=False, server_default=sa.text("'{}'")),
        schema=SCHEMA,
    )

    # 5. processed_events — durable consumer dedupe (work17 DbDedupe). Byte-identical to the shared
    # idempotency-python migration; folded here (one per service, in the service's own schema).
    op.create_table(
        "processed_events",
        sa.Column("scope", sa.Text(), nullable=False),
        sa.Column("dedupe_key", sa.Text(), nullable=False),
        sa.Column(
            "processed_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.PrimaryKeyConstraint("scope", "dedupe_key"),
        schema=SCHEMA,
    )

    # 6. refund_events — the durable outbox the work15 relay drains (entity_id is the refund/dispute id).
    op.create_table(
        "refund_events",
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
        "ix_refund_events_unpublished",
        "refund_events",
        ["id"],
        schema=SCHEMA,
        postgresql_where=sa.text("published_at IS NULL"),
    )


def downgrade() -> None:
    op.drop_index("ix_refund_events_unpublished", table_name="refund_events", schema=SCHEMA)
    op.drop_table("refund_events", schema=SCHEMA)
    op.drop_table("processed_events", schema=SCHEMA)
    op.drop_table("verified_paylinks", schema=SCHEMA)
    op.drop_index("ix_evidence_dispute", table_name="dispute_evidence", schema=SCHEMA)
    op.drop_table("dispute_evidence", schema=SCHEMA)
    op.drop_index("ix_disputes_payment", table_name="disputes", schema=SCHEMA)
    op.drop_index("ix_disputes_due", table_name="disputes", schema=SCHEMA)
    op.drop_table("disputes", schema=SCHEMA)
    op.drop_index("ix_refunds_org", table_name="refunds", schema=SCHEMA)
    op.drop_index("ix_refunds_state", table_name="refunds", schema=SCHEMA)
    op.drop_index("ix_refunds_payment", table_name="refunds", schema=SCHEMA)
    op.drop_table("refunds", schema=SCHEMA)
