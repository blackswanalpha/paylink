"""SQLAlchemy declarative base. All tables live in the service-owned ``invoice`` schema."""

from __future__ import annotations

from sqlalchemy import MetaData
from sqlalchemy.orm import DeclarativeBase

SCHEMA = "invoice"


class Base(DeclarativeBase):
    metadata = MetaData(schema=SCHEMA)
