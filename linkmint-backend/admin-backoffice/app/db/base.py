"""SQLAlchemy declarative base. All tables live in the service-owned ``admin`` schema."""

from __future__ import annotations

from sqlalchemy import MetaData
from sqlalchemy.orm import DeclarativeBase

SCHEMA = "admin"


class Base(DeclarativeBase):
    metadata = MetaData(schema=SCHEMA)
