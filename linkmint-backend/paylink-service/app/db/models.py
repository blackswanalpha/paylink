"""ORM models for the ``paylink`` schema (mirrors backendfeatures.md §2.2 DDL).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types.
"""

from datetime import datetime
from decimal import Decimal
from typing import Any

from sqlalchemy import BigInteger, DateTime, ForeignKey, Index, Integer, Numeric, String, text
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class PayLinkRow(Base):
    __tablename__ = "paylinks"
    # Fetch server defaults (created_at/updated_at/vote_count) back onto the instance after INSERT.
    __mapper_args__ = {"eager_defaults": True}

    pl_id: Mapped[str] = mapped_column(String, primary_key=True)
    creator_addr: Mapped[str] = mapped_column(String, nullable=False)
    receiver_addr: Mapped[str] = mapped_column(String, nullable=False)
    owner_addr: Mapped[str] = mapped_column(String, nullable=False)
    amount: Mapped[Decimal] = mapped_column(Numeric(38, 0), nullable=False)
    currency: Mapped[str] = mapped_column(String, nullable=False)
    status: Mapped[str] = mapped_column(String, nullable=False)
    expiry: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    usage: Mapped[str] = mapped_column(String, nullable=False)
    # Python attr is `meta` (avoids the reserved `metadata`); DB column stays `metadata`.
    meta: Mapped[dict[str, Any] | None] = mapped_column("metadata", JSONB, nullable=True)
    rules: Mapped[Any | None] = mapped_column(JSONB, nullable=True)
    chain_tx_hash: Mapped[str | None] = mapped_column(String, nullable=True)
    vote_count: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default=text("0")
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        nullable=False,
        server_default=text("now()"),
        onupdate=text("now()"),
    )
    verified_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    __table_args__ = (
        Index("paylinks_creator_idx", "creator_addr", "status", text("created_at DESC")),
        Index("paylinks_receiver_idx", "receiver_addr", "status", text("created_at DESC")),
        Index(
            "paylinks_expiry_idx",
            "expiry",
            postgresql_where=text("status = 'PENDING'"),
        ),
    )


class PayLinkEventRow(Base):
    __tablename__ = "paylink_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    # Within-schema FK only (no cross-schema FKs per standard.md).
    pl_id: Mapped[str] = mapped_column(String, ForeignKey("paylink.paylinks.pl_id"), nullable=False)
    kind: Mapped[str] = mapped_column(String, nullable=False)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
