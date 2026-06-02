"""Migrations apply cleanly from an empty database and round-trip down to base."""

from __future__ import annotations

import os
from pathlib import Path

import pytest
import sqlalchemy as sa

pytestmark = pytest.mark.integration

SERVICE_ROOT = Path(__file__).resolve().parents[2]


def _psycopg_url(raw: str) -> str:
    if "+psycopg2" in raw:
        return raw.replace("+psycopg2", "+psycopg")
    if "+psycopg" not in raw:
        return raw.replace("postgresql://", "postgresql+psycopg://", 1)
    return raw


def test_upgrade_from_empty_then_downgrade() -> None:
    pg_mod = pytest.importorskip("testcontainers.postgres")
    try:
        container = pg_mod.PostgresContainer("postgres:16")
        container.start()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Docker/Postgres unavailable: {exc}")

    try:
        url = _psycopg_url(container.get_connection_url())
        os.environ["INVOICE_DATABASE_URL"] = url
        from alembic.config import Config

        from alembic import command

        cfg = Config(str(SERVICE_ROOT / "alembic.ini"))
        cfg.set_main_option("script_location", str(SERVICE_ROOT / "alembic"))

        command.upgrade(cfg, "head")

        engine = sa.create_engine(url)
        with engine.connect() as conn:
            schemas = set(
                conn.execute(
                    sa.text("SELECT schema_name FROM information_schema.schemata")
                ).scalars()
            )
            assert "invoice" in schemas

            tables = set(
                conn.execute(
                    sa.text(
                        "SELECT table_name FROM information_schema.tables "
                        "WHERE table_schema = 'invoice'"
                    )
                ).scalars()
            )
            assert {"invoices", "invoice_lines", "invoice_events"} <= tables
            # Subscriptions are deferred to work31 — the table must NOT exist yet.
            assert "subscriptions" not in tables

            indexes = set(
                conn.execute(
                    sa.text("SELECT indexname FROM pg_indexes WHERE schemaname = 'invoice'")
                ).scalars()
            )
            assert {
                "ix_invoices_merchant_id",
                "uq_invoices_pl_id",
                "ix_invoices_status_due",
                "ix_invoice_lines_invoice_id",
                "ix_invoice_events_unpublished",
            } <= indexes
        engine.dispose()

        command.downgrade(cfg, "base")
        engine = sa.create_engine(url)
        with engine.connect() as conn:
            tables = set(
                conn.execute(
                    sa.text(
                        "SELECT table_name FROM information_schema.tables "
                        "WHERE table_schema = 'invoice'"
                    )
                ).scalars()
            )
            assert "invoices" not in tables
        engine.dispose()
    finally:
        container.stop()
        os.environ.pop("INVOICE_DATABASE_URL", None)
