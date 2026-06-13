"""The alembic migration applies and creates the `refund` schema's tables."""

from __future__ import annotations

import pytest
from sqlalchemy import create_engine, inspect

pytestmark = pytest.mark.integration


def test_schema_tables_exist(pg_url: str) -> None:
    engine = create_engine(pg_url.replace("+psycopg", "+psycopg"))
    insp = inspect(engine)
    tables = set(insp.get_table_names(schema="refund"))
    assert {
        "refunds",
        "disputes",
        "dispute_evidence",
        "verified_paylinks",
        "processed_events",
        "refund_events",
    }.issubset(tables)
    engine.dispose()
