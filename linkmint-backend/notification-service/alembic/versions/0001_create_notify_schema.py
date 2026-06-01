"""create notify schema: webhooks (forward), deliveries (+retry index), templates (+seed)

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

SCHEMA = "notify"

# Phase-1 templates ({channel}.{event_snake}.{locale}). Placeholders are $-style (string.Template).
_SEED_TEMPLATES: list[dict[str, object]] = [
    {
        "template_id": "sms.paylink_verified.en",
        "channel": "sms",
        "locale": "en",
        "body": "Your PayLink for $amount $currency is verified. Ref: $paylink_id",
        "version": 1,
        "active": True,
    },
    {
        "template_id": "email.paylink_verified.en",
        "channel": "email",
        "locale": "en",
        "body": "Hi — your PayLink ($paylink_id) for $amount $currency has been verified.",
        "version": 1,
        "active": True,
    },
    {
        "template_id": "sms.payment_failed.en",
        "channel": "sms",
        "locale": "en",
        "body": "Payment of $amount $currency failed: $reason.",
        "version": 1,
        "active": True,
    },
    {
        "template_id": "email.payment_failed.en",
        "channel": "email",
        "locale": "en",
        "body": "Your payment of $amount $currency could not be completed. Reason: $reason.",
        "version": 1,
        "active": True,
    },
]


def upgrade() -> None:
    op.execute(f"CREATE SCHEMA IF NOT EXISTS {SCHEMA}")

    # Phase-2 forward-schema: merchant webhook registration + HMAC delivery (unused in Phase 1).
    op.create_table(
        "webhooks",
        sa.Column("webhook_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column("merchant_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("url", sa.Text(), nullable=False),
        sa.Column("events", postgresql.ARRAY(sa.Text()), nullable=False),
        sa.Column("secret", sa.Text(), nullable=False),  # KMS-encrypted (Phase 2)
        sa.Column("status", sa.Text(), nullable=False),  # ACTIVE|PAUSED|REVOKED
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        schema=SCHEMA,
    )

    op.create_table(
        "deliveries",
        sa.Column("delivery_id", postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column(
            "webhook_id",
            postgresql.UUID(as_uuid=True),
            sa.ForeignKey(f"{SCHEMA}.webhooks.webhook_id"),
            nullable=True,
        ),
        sa.Column("channel", sa.Text(), nullable=False),  # sms|email|push|webhook
        sa.Column("recipient", sa.Text(), nullable=False),
        sa.Column("event_kind", sa.Text(), nullable=False),
        sa.Column("payload", postgresql.JSONB(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),  # QUEUED|SENT|FAILED|EXHAUSTED
        sa.Column("attempts", sa.Integer(), nullable=False, server_default=sa.text("0")),
        sa.Column("last_error", sa.Text(), nullable=True),
        sa.Column("next_retry_at", sa.TIMESTAMP(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("delivered_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    # Partial index for the (Phase-2) sweeper that recovers orphaned retryable rows.
    op.create_index(
        "deliveries_retry_idx",
        "deliveries",
        ["next_retry_at"],
        schema=SCHEMA,
        postgresql_where=sa.text("status IN ('QUEUED', 'FAILED')"),
    )
    # Anti-replay (A.7 analog): one event → one delivery. The DB is the arbiter of the per-event
    # dedupe key (set in payload by NotificationService.intake) so a concurrent at-least-once
    # redelivery — the work15 bus path, which has no Idempotency-Key — cannot race two inserts past
    # the read-then-insert check. The intake catches the conflict and reuses the existing delivery.
    op.execute(
        "CREATE UNIQUE INDEX deliveries_dedupe_uidx "
        "ON notify.deliveries ((payload->>'dedupe_key'))"
    )

    op.create_table(
        "templates",
        sa.Column("template_id", sa.Text(), primary_key=True),  # sms.paylink_verified.en
        sa.Column("channel", sa.Text(), nullable=False),
        sa.Column("locale", sa.Text(), nullable=False),
        sa.Column("body", sa.Text(), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column("active", sa.Boolean(), nullable=False, server_default=sa.text("true")),
        schema=SCHEMA,
    )

    templates = sa.table(
        "templates",
        sa.column("template_id", sa.Text),
        sa.column("channel", sa.Text),
        sa.column("locale", sa.Text),
        sa.column("body", sa.Text),
        sa.column("version", sa.Integer),
        sa.column("active", sa.Boolean),
        schema=SCHEMA,
    )
    op.bulk_insert(templates, _SEED_TEMPLATES)


def downgrade() -> None:
    op.drop_table("templates", schema=SCHEMA)
    op.execute("DROP INDEX IF EXISTS notify.deliveries_dedupe_uidx")
    op.drop_index("deliveries_retry_idx", table_name="deliveries", schema=SCHEMA)
    op.drop_table("deliveries", schema=SCHEMA)
    op.drop_table("webhooks", schema=SCHEMA)
