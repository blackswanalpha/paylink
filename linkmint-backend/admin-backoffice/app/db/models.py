"""ORM models for the ``admin`` schema.

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real types.

Phase 1 owns only ``admin.staff`` — the default-deny scope grants keyed by the JWT ``sub`` (an
opaque ref to ``identity.users`` — NO cross-schema FK). The Phase-2 ``admin.feature_flags`` and
``admin.announcements`` tables (spec §2.18) are created by the migration but have no Phase-1 ORM
surface — the read-only console never touches them.
"""

import uuid
from datetime import datetime

from sqlalchemy import Text, text
from sqlalchemy.dialects.postgresql import ARRAY, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class StaffRow(Base):
    __tablename__ = "staff"

    # Opaque ref to identity.users (the JWT ``sub``) — NO cross-schema FK.
    sub: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    scopes: Mapped[list[str]] = mapped_column(
        ARRAY(Text), nullable=False, server_default=text("'{}'")
    )
    note: Mapped[str | None] = mapped_column(Text, nullable=True)
    updated_at: Mapped[datetime] = mapped_column(nullable=False, server_default=text("now()"))
