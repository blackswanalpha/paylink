"""create notify.inbox_notifications (in-app notification center, address-scoped + dedupe)

Revision ID: 0002
Revises: 0001
Create Date: 2026-06-02
"""

from collections.abc import Sequence

import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision: str = "0002"
down_revision: str | None = "0001"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

SCHEMA = "notify"


def upgrade() -> None:
    op.create_table(
        "inbox_notifications",
        sa.Column("notification_id", postgresql.UUID(as_uuid=True), primary_key=True),
        # The recipient ADDRESS (lowercased creator/merchant address the gateway injects as
        # X-Creator-Addr) — the per-tenant scope key for the read API. Distinct from the UUID
        # user_id that keys SMS/email deliveries.
        sa.Column("recipient_addr", sa.Text(), nullable=False),
        sa.Column("kind", sa.Text(), nullable=False),  # success|info|warning|error
        sa.Column("title", sa.Text(), nullable=False),
        sa.Column("body", sa.Text(), nullable=True),
        sa.Column("href", sa.Text(), nullable=True),
        sa.Column("event_kind", sa.Text(), nullable=True),
        sa.Column("dedupe_key", sa.Text(), nullable=False),
        sa.Column("read", sa.Boolean(), nullable=False, server_default=sa.text("false")),
        sa.Column(
            "created_at",
            sa.TIMESTAMP(timezone=True),
            nullable=False,
            server_default=sa.text("now()"),
        ),
        sa.Column("read_at", sa.TIMESTAMP(timezone=True), nullable=True),
        schema=SCHEMA,
    )
    # Listing is "newest first, scoped to the caller" — a composite covering the keyset cursor
    # (created_at, notification_id).
    op.create_index(
        "inbox_recipient_created_idx",
        "inbox_notifications",
        ["recipient_addr", sa.text("created_at DESC"), sa.text("notification_id DESC")],
        schema=SCHEMA,
    )
    # Anti-replay (mirrors deliveries_dedupe_uidx): one (event, recipient, source) → one inbox row,
    # so an at-least-once producer (paylink-service emit / the bus) can't double-post.
    op.create_index(
        "inbox_dedupe_uidx",
        "inbox_notifications",
        ["dedupe_key"],
        unique=True,
        schema=SCHEMA,
    )


def downgrade() -> None:
    op.drop_index("inbox_dedupe_uidx", table_name="inbox_notifications", schema=SCHEMA)
    op.drop_index("inbox_recipient_created_idx", table_name="inbox_notifications", schema=SCHEMA)
    op.drop_table("inbox_notifications", schema=SCHEMA)
