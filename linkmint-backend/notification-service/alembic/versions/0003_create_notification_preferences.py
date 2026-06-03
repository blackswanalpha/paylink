"""create notify.notification_preferences (per-recipient channel + event opt-outs)

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

SCHEMA = "notify"


def upgrade() -> None:
    op.create_table(
        "notification_preferences",
        sa.Column("preference_id", postgresql.UUID(as_uuid=True), primary_key=True),
        # The creator/merchant ADDRESS (lowercased X-Creator-Addr) — the same per-tenant scope key
        # the inbox uses. One row per recipient; a missing row means "all enabled" (opt-out).
        sa.Column("recipient_addr", sa.Text(), nullable=False),
        # {channel: bool} for in_app/email/sms, {event_kind: bool} for the paylink.*/payment.* kinds.
        sa.Column(
            "channels", postgresql.JSONB(), nullable=False, server_default=sa.text("'{}'::jsonb")
        ),
        sa.Column(
            "events", postgresql.JSONB(), nullable=False, server_default=sa.text("'{}'::jsonb")
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
        schema=SCHEMA,
    )
    # One preferences row per recipient — the upsert's arbiter.
    op.create_index(
        "notification_preferences_recipient_uidx",
        "notification_preferences",
        ["recipient_addr"],
        unique=True,
        schema=SCHEMA,
    )


def downgrade() -> None:
    op.drop_index(
        "notification_preferences_recipient_uidx",
        table_name="notification_preferences",
        schema=SCHEMA,
    )
    op.drop_table("notification_preferences", schema=SCHEMA)
