"""SQLAlchemy declarative base. All tables live in the service-owned ``pricing`` schema."""

from __future__ import annotations

from sqlalchemy import MetaData
from sqlalchemy.orm import DeclarativeBase

SCHEMA = "pricing"


class Base(DeclarativeBase):
    metadata = MetaData(schema=SCHEMA)
