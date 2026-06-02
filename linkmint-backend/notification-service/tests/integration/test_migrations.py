"""The Alembic migration creates the notify schema, tables, retry index, and seeds templates."""

from __future__ import annotations

import pytest
from sqlalchemy import create_engine, text

pytestmark = pytest.mark.integration


def test_schema_tables_index_and_seed(pg_url: str) -> None:
    engine = create_engine(pg_url, future=True)
    try:
        with engine.connect() as conn:
            tables = set(
                conn.execute(
                    text(
                        "SELECT table_name FROM information_schema.tables WHERE table_schema='notify'"
                    )
                ).scalars()
            )
            assert {"webhooks", "deliveries", "templates"} <= tables

            index = conn.execute(
                text(
                    "SELECT indexdef FROM pg_indexes "
                    "WHERE schemaname='notify' AND indexname='deliveries_retry_idx'"
                )
            ).scalar_one()
            assert "next_retry_at" in index
            assert "status" in index  # partial WHERE predicate

            template_count = conn.execute(
                text("SELECT count(*) FROM notify.templates WHERE active")
            ).scalar_one()
            assert template_count == 4

            sms = conn.execute(
                text(
                    "SELECT body FROM notify.templates WHERE template_id='sms.paylink_verified.en'"
                )
            ).scalar_one()
            assert "$amount" in sms
    finally:
        engine.dispose()
