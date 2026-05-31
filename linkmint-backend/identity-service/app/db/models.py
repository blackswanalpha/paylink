"""ORM models for the ``identity`` schema (mirrors backendfeatures.md §2.9 DDL).

NOTE: ``from __future__ import annotations`` is intentionally NOT used here — SQLAlchemy 2.0
resolves ``Mapped[...]`` annotations at class-creation time and is most robust with real
(non-stringized) types. FKs are within the ``identity`` schema only (no cross-schema FKs).
"""

import uuid
from datetime import datetime
from typing import Any

from sqlalchemy import BigInteger, DateTime, ForeignKey, SmallInteger, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, INET, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class UserRow(Base):
    __tablename__ = "users"
    __mapper_args__ = {"eager_defaults": True}

    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    email: Mapped[str | None] = mapped_column(Text, nullable=True)
    phone: Mapped[str | None] = mapped_column(Text, nullable=True)
    password_hash: Mapped[str | None] = mapped_column(
        Text, nullable=True
    )  # argon2id; null for OAuth-only
    kyc_tier: Mapped[int] = mapped_column(SmallInteger, nullable=False, server_default=text("0"))
    status: Mapped[str] = mapped_column(Text, nullable=False)  # ACTIVE|SUSPENDED|DELETED
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    last_login_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class OAuthIdentityRow(Base):
    __tablename__ = "oauth_identities"

    provider: Mapped[str] = mapped_column(Text, primary_key=True)  # google|apple|github
    subject: Mapped[str] = mapped_column(Text, primary_key=True)
    user_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.users.user_id"), nullable=False
    )


class MfaFactorRow(Base):
    __tablename__ = "mfa_factors"

    user_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.users.user_id"), primary_key=True
    )
    kind: Mapped[str] = mapped_column(Text, primary_key=True)  # totp|webauthn|sms_otp
    secret: Mapped[str] = mapped_column(Text, nullable=False)  # AES-GCM ciphertext (KMS stand-in)
    enrolled_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    # NULL until the enrollment is verified; a non-null value means the factor is active.
    activated_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class OrganizationRow(Base):
    __tablename__ = "organizations"
    __mapper_args__ = {"eager_defaults": True}

    org_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    name: Mapped[str] = mapped_column(Text, nullable=False)
    type: Mapped[str] = mapped_column(Text, nullable=False)  # merchant|developer|admin
    created_by: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.users.user_id"), nullable=False
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )


class MembershipRow(Base):
    __tablename__ = "memberships"

    org_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.organizations.org_id"), primary_key=True
    )
    user_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.users.user_id"), primary_key=True
    )
    role: Mapped[str] = mapped_column(Text, nullable=False)


class ApiKeyRow(Base):
    __tablename__ = "api_keys"
    __mapper_args__ = {"eager_defaults": True}

    api_key_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    org_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.organizations.org_id"), nullable=False
    )
    name: Mapped[str] = mapped_column(Text, nullable=False)
    prefix: Mapped[str] = mapped_column(Text, nullable=False)  # displayed, e.g. lm_live_AbCd1234
    hash: Mapped[str] = mapped_column(Text, nullable=False)  # argon2id of the full key
    scopes: Mapped[list[str]] = mapped_column(ARRAY(Text), nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False)  # ACTIVE|REVOKED
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class SessionRow(Base):
    __tablename__ = "sessions"
    __mapper_args__ = {"eager_defaults": True}

    session_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    user_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("identity.users.user_id"), nullable=False
    )
    refresh_token: Mapped[str] = mapped_column(Text, nullable=False)  # SHA-256 hash of the token
    user_agent: Mapped[str | None] = mapped_column(Text, nullable=True)
    ip: Mapped[str | None] = mapped_column(INET, nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)


class IdentityEventRow(Base):
    """Durable outbox — the source of truth work15 will drain onto Kafka/SQS."""

    __tablename__ = "identity_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    subject_type: Mapped[str] = mapped_column(Text, nullable=False)  # user|org|api_key|...
    subject_id: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    kind: Mapped[str] = mapped_column(Text, nullable=False)  # logical event name (identity.*)
    payload: Mapped[dict[str, Any]] = mapped_column(JSONB, nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=text("now()")
    )
