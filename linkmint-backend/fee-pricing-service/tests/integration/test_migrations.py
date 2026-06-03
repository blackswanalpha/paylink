"""Migrations apply cleanly from an empty database (with seeds) and round-trip down to base."""

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
        os.environ["PRICING_DATABASE_URL"] = url
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
            assert "pricing" in schemas

            tables = set(
                conn.execute(
                    sa.text(
                        "SELECT table_name FROM information_schema.tables "
                        "WHERE table_schema = 'pricing'"
                    )
                ).scalars()
            )
            assert {
                "tiers",
                "rail_fee_schedules",
                "merchant_pricing",
                "fx_rates",
                "quotes",
                "platform_fee_accruals",
                "platform_fee_invoices",
                "pricing_events",
            } <= tables

            indexes = set(
                conn.execute(
                    sa.text("SELECT indexname FROM pg_indexes WHERE schemaname = 'pricing'")
                ).scalars()
            )
            assert {
                "ix_rail_fee_rail",
                "ix_fx_pair_time",
                "ix_quotes_merchant",
                "ix_accrual_unbilled",
                "ix_pfi_merchant",
                "ix_pricing_events_unpublished",
            } <= indexes

            # Seed data: a fresh DB can quote immediately (5 tiers + 4 global rail schedules).
            tier_count = conn.execute(sa.text("SELECT count(*) FROM pricing.tiers")).scalar_one()
            assert tier_count == 5
            rail_count = conn.execute(
                sa.text("SELECT count(*) FROM pricing.rail_fee_schedules")
            ).scalar_one()
            assert rail_count == 4
            standard_bps = conn.execute(
                sa.text("SELECT platform_pct_bps FROM pricing.tiers WHERE tier = 'standard'")
            ).scalar_one()
            assert standard_bps == 250
        engine.dispose()

        command.downgrade(cfg, "base")
        engine = sa.create_engine(url)
        with engine.connect() as conn:
            tables = set(
                conn.execute(
                    sa.text(
                        "SELECT table_name FROM information_schema.tables "
                        "WHERE table_schema = 'pricing'"
                    )
                ).scalars()
            )
            assert "tiers" not in tables
        engine.dispose()
    finally:
        container.stop()
        os.environ.pop("PRICING_DATABASE_URL", None)
