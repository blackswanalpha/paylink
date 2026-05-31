"""Shared test doubles + helpers (imported by conftest and integration tests).

The fake repository is an in-memory mirror of :class:`IdentityRepository` with the same surface, so
the unit/API suite exercises the real services, security primitives, JWT, RBAC, and idempotency
without Docker. A single RSA keypair is generated once (module load) and injected via settings so the
KeyStore is deterministic and fast across tests.
"""

from __future__ import annotations

import base64
import uuid
from datetime import UTC, datetime
from typing import Any

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.config import Settings
from app.db.models import (
    ApiKeyRow,
    MembershipRow,
    MfaFactorRow,
    OAuthIdentityRow,
    OrganizationRow,
    SessionRow,
    UserRow,
)

# One ephemeral RSA keypair for the whole test run (avoids per-test keygen cost).
_TEST_KEY = rsa.generate_private_key(public_exponent=65537, key_size=2048)
TEST_PRIVATE_PEM = _TEST_KEY.private_bytes(
    serialization.Encoding.PEM,
    serialization.PrivateFormat.PKCS8,
    serialization.NoEncryption(),
).decode()

# Fixed 32-byte MFA encryption key (deterministic).
TEST_MFA_KEY = base64.b64encode(bytes(range(32))).decode()


async def noop_commit() -> None:
    return None


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "jwt_private_key_pem": TEST_PRIVATE_PEM,
        "mfa_encryption_key": TEST_MFA_KEY,
        "argon2_time_cost": 1,
        "argon2_memory_cost_kib": 8,
        "argon2_parallelism": 1,
        "oauth_fake": True,
        "event_publisher_mode": "noop",
        "auth_failed_threshold": 3,
    }
    base.update(overrides)
    return Settings(**base)


class FakeRepository:
    """In-memory IdentityRepository with matching method names/semantics."""

    def __init__(self) -> None:
        self.users: dict[uuid.UUID, UserRow] = {}
        self.oauth: dict[tuple[str, str], OAuthIdentityRow] = {}
        self.mfa: dict[tuple[uuid.UUID, str], MfaFactorRow] = {}
        self.orgs: dict[uuid.UUID, OrganizationRow] = {}
        self.memberships: dict[tuple[uuid.UUID, uuid.UUID], MembershipRow] = {}
        self.api_keys: dict[uuid.UUID, ApiKeyRow] = {}
        self.sessions: dict[uuid.UUID, SessionRow] = {}
        self.events: list[tuple[str, uuid.UUID | None, str, dict[str, Any]]] = []

    # ── users ──
    async def insert_user(self, row: UserRow) -> UserRow:
        if row.kyc_tier is None:
            row.kyc_tier = 0
        if row.created_at is None:
            row.created_at = datetime.now(UTC)
        self.users[row.user_id] = row
        return row

    async def get_user(self, user_id: uuid.UUID) -> UserRow | None:
        return self.users.get(user_id)

    async def get_user_by_email(self, email: str) -> UserRow | None:
        return next((u for u in self.users.values() if u.email == email), None)

    async def get_user_by_phone(self, phone: str) -> UserRow | None:
        return next((u for u in self.users.values() if u.phone == phone), None)

    # ── oauth ──
    async def get_oauth_identity(self, provider: str, subject: str) -> OAuthIdentityRow | None:
        return self.oauth.get((provider, subject))

    async def insert_oauth_identity(self, row: OAuthIdentityRow) -> OAuthIdentityRow:
        self.oauth[(row.provider, row.subject)] = row
        return row

    # ── mfa ──
    async def get_mfa_factor(self, user_id: uuid.UUID, kind: str) -> MfaFactorRow | None:
        return self.mfa.get((user_id, kind))

    async def insert_mfa_factor(self, row: MfaFactorRow) -> MfaFactorRow:
        if row.enrolled_at is None:
            row.enrolled_at = datetime.now(UTC)
        self.mfa[(row.user_id, row.kind)] = row
        return row

    async def delete_mfa_factor(self, row: MfaFactorRow) -> None:
        self.mfa.pop((row.user_id, row.kind), None)

    async def list_active_mfa(self, user_id: uuid.UUID) -> list[MfaFactorRow]:
        return [
            f for (uid, _), f in self.mfa.items() if uid == user_id and f.activated_at is not None
        ]

    # ── orgs ──
    async def insert_org(self, row: OrganizationRow) -> OrganizationRow:
        if row.created_at is None:
            row.created_at = datetime.now(UTC)
        self.orgs[row.org_id] = row
        return row

    async def get_org(self, org_id: uuid.UUID) -> OrganizationRow | None:
        return self.orgs.get(org_id)

    # ── memberships ──
    async def insert_membership(self, row: MembershipRow) -> MembershipRow:
        self.memberships[(row.org_id, row.user_id)] = row
        return row

    async def get_membership(self, org_id: uuid.UUID, user_id: uuid.UUID) -> MembershipRow | None:
        return self.memberships.get((org_id, user_id))

    async def list_members(self, org_id: uuid.UUID) -> list[MembershipRow]:
        return [m for (oid, _), m in self.memberships.items() if oid == org_id]

    async def list_memberships_for_user(self, user_id: uuid.UUID) -> list[MembershipRow]:
        return [m for (_, uid), m in self.memberships.items() if uid == user_id]

    async def count_owners(self, org_id: uuid.UUID) -> int:
        return sum(
            1 for (oid, _), m in self.memberships.items() if oid == org_id and m.role == "owner"
        )

    async def delete_membership(self, row: MembershipRow) -> None:
        self.memberships.pop((row.org_id, row.user_id), None)

    # ── api keys ──
    async def insert_api_key(self, row: ApiKeyRow) -> ApiKeyRow:
        if row.created_at is None:
            row.created_at = datetime.now(UTC)
        self.api_keys[row.api_key_id] = row
        return row

    async def get_api_key(self, api_key_id: uuid.UUID) -> ApiKeyRow | None:
        return self.api_keys.get(api_key_id)

    async def list_api_keys_for_org(self, org_id: uuid.UUID) -> list[ApiKeyRow]:
        rows = [k for k in self.api_keys.values() if k.org_id == org_id]
        return sorted(rows, key=lambda k: k.created_at, reverse=True)

    async def list_api_keys_by_prefix(self, prefix: str) -> list[ApiKeyRow]:
        return [k for k in self.api_keys.values() if k.prefix == prefix and k.status == "ACTIVE"]

    # ── sessions ──
    async def insert_session(self, row: SessionRow) -> SessionRow:
        if row.created_at is None:
            row.created_at = datetime.now(UTC)
        self.sessions[row.session_id] = row
        return row

    async def get_session(self, session_id: uuid.UUID) -> SessionRow | None:
        return self.sessions.get(session_id)

    async def get_session_by_refresh_hash(self, token_hash: str) -> SessionRow | None:
        return next((s for s in self.sessions.values() if s.refresh_token == token_hash), None)

    async def list_sessions_for_user(self, user_id: uuid.UUID) -> list[SessionRow]:
        rows = [s for s in self.sessions.values() if s.user_id == user_id]
        return sorted(rows, key=lambda s: s.created_at, reverse=True)

    async def list_active_sessions_for_user(self, user_id: uuid.UUID) -> list[SessionRow]:
        return [s for s in self.sessions.values() if s.user_id == user_id and s.revoked_at is None]

    # ── events ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self.events.append((subject_type, subject_id, kind, payload))
