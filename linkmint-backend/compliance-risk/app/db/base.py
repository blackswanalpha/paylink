"""SQLAlchemy declarative base. All tables live in the service-owned ``compliance`` schema."""

from __future__ import annotations

from sqlalchemy import MetaData
from sqlalchemy.orm import DeclarativeBase

SCHEMA = "compliance"


class Base(DeclarativeBase):
    metadata = MetaData(schema=SCHEMA)
