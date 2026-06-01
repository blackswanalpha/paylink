"""Shared test doubles + helpers (imported by conftest and integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``, the way identity-service would) and its PUBLIC PEM is injected via settings, so
compliance-risk's verifier-only JWT seam, the self-or-admin authz, the AES-GCM provider cipher, the
HMAC callback verification, and idempotency are all exercised with REAL primitives — no issuer code
here (compliance-risk is a consumer). The fake repository is an in-memory mirror of
:class:`ComplianceRepository` with the same surface, so the unit/API suite runs without Docker.
"""

from __future__ import annotations

import base64
import uuid
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from typing import Any

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.config import Settings
from app.db.models import (
    ActivityEventRow,
    FlagRow,
    KycRecordRow,
    RiskScoreRow,
)

# One ephemeral RSA keypair for the whole test run (avoids per-test keygen cost). The private key
# stands in for identity-service's signer; the public PEM is what compliance-risk verifies with.
_TEST_KEY = rsa.generate_private_key(public_exponent=65537, key_size=2048)
TEST_PRIVATE_PEM = _TEST_KEY.private_bytes(
    serialization.Encoding.PEM,
    serialization.PrivateFormat.PKCS8,
    serialization.NoEncryption(),
).decode()
TEST_PUBLIC_PEM = (
    _TEST_KEY.public_key()
    .public_bytes(serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo)
    .decode()
)

# Fixed 32-byte provider-ref encryption key (deterministic).
TEST_PROVIDER_KEY = base64.b64encode(bytes(range(32))).decode()

ISSUER = "linkmint-identity"
AUDIENCE = "linkmint"
CALLBACK_SECRET = "devnet-callback-secret"


async def noop_commit() -> None:
    return None


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "jwt_public_key_pem": TEST_PUBLIC_PEM,
        "jwt_issuer": ISSUER,
        "jwt_audience": AUDIENCE,
        "provider_encryption_key": TEST_PROVIDER_KEY,
        "callback_secrets": f"stub:{CALLBACK_SECRET}",
        "event_publisher_mode": "noop",
    }
    base.update(overrides)
    return Settings(**base)


def mint_token(
    *,
    user_id: str | None = None,
    roles: list[tuple[str, str]] | None = None,
    user_roles: list[str] | None = None,
    kyc_tier: int = 1,
    sid: str = "s1",
    issuer: str = ISSUER,
    audience: str = AUDIENCE,
    private_pem: str | None = None,
    algorithm: str = "RS256",
    expired: bool = False,
) -> str:
    """Mint an RS256 access token the way identity-service would (for the compliance verifier)."""
    now = datetime.now(UTC)
    if expired:
        now = now - timedelta(hours=2)
    exp = now + timedelta(hours=1)
    payload = {
        "sub": user_id or str(uuid.uuid4()),
        "iss": issuer,
        "aud": audience,
        "iat": int(now.timestamp()),
        "nbf": int(now.timestamp()),
        "exp": int(exp.timestamp()),
        "jti": uuid.uuid4().hex,
        "sid": sid,
        "roles": [{"org_id": o, "role": r} for o, r in (roles or [])],
        "user_roles": user_roles if user_roles is not None else ["payer"],
        "kyc_tier": kyc_tier,
    }
    key = private_pem if private_pem is not None else TEST_PRIVATE_PEM
    return pyjwt.encode(payload, key, algorithm=algorithm)


def user_headers(user_id: str, *, admin: bool = False) -> dict[str, str]:
    """Bearer header for ``user_id`` (optionally with an admin user-role)."""
    token = mint_token(user_id=user_id, user_roles=["admin"] if admin else ["payer"])
    return {"Authorization": f"Bearer {token}"}


class FakeRepository:
    """In-memory ComplianceRepository with matching method names/semantics."""

    def __init__(self) -> None:
        self.kyc: dict[uuid.UUID, KycRecordRow] = {}
        self.risk_scores: list[RiskScoreRow] = []
        self.flags: list[FlagRow] = []
        self.activity: list[ActivityEventRow] = []
        self.events: list[tuple[str, uuid.UUID | None, str, dict[str, Any]]] = []
        self._risk_seq = 0
        self._flag_seq = 0
        self._activity_seq = 0

    # ── kyc records ──
    async def get_kyc_record(self, user_id: uuid.UUID) -> KycRecordRow | None:
        return self.kyc.get(user_id)

    async def upsert_kyc_record(self, row: KycRecordRow) -> KycRecordRow:
        if row.tier is None:
            row.tier = 0
        self.kyc[row.user_id] = row
        return row

    async def get_tier(self, user_id: uuid.UUID) -> int:
        record = self.kyc.get(user_id)
        return int(record.tier) if record is not None else 0

    # ── risk scores ──
    async def insert_risk_score(self, row: RiskScoreRow) -> RiskScoreRow:
        self._risk_seq += 1
        row.id = self._risk_seq
        if row.evaluated_at is None:
            row.evaluated_at = datetime.now(UTC)
        self.risk_scores.append(row)
        return row

    async def latest_risk_score(self, user_id: uuid.UUID) -> RiskScoreRow | None:
        rows = [r for r in self.risk_scores if r.user_id == user_id]
        if not rows:
            return None
        return max(rows, key=lambda r: (r.evaluated_at, r.id))

    # ── flags ──
    async def insert_flag(self, row: FlagRow) -> FlagRow:
        self._flag_seq += 1
        row.id = self._flag_seq
        if row.raised_at is None:
            row.raised_at = datetime.now(UTC)
        self.flags.append(row)
        return row

    async def list_open_flags(self, user_id: uuid.UUID) -> list[FlagRow]:
        rows = [f for f in self.flags if f.user_id == user_id and f.resolved_at is None]
        return sorted(rows, key=lambda f: f.raised_at, reverse=True)

    async def count_flags(self, user_id: uuid.UUID) -> int:
        return sum(1 for f in self.flags if f.user_id == user_id)

    # ── activity events ──
    async def insert_activity(self, row: ActivityEventRow) -> ActivityEventRow:
        self._activity_seq += 1
        row.id = self._activity_seq
        if row.occurred_at is None:
            row.occurred_at = datetime.now(UTC)
        self.activity.append(row)
        return row

    async def count_activity_since(self, user_id: uuid.UUID, since: datetime) -> int:
        return sum(1 for a in self.activity if a.user_id == user_id and a.occurred_at >= since)

    async def sum_amount_since(self, user_id: uuid.UUID, since: datetime) -> Decimal:
        total = Decimal(0)
        for a in self.activity:
            if a.user_id == user_id and a.occurred_at >= since and a.amount is not None:
                total += a.amount
        return total

    # ── events (outbox) ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self.events.append((subject_type, subject_id, kind, payload))

    # ── test helpers ──
    def seed_kyc(self, user_id: uuid.UUID, tier: int) -> None:
        self.kyc[user_id] = KycRecordRow(user_id=user_id, tier=tier)

    def seed_activity(
        self,
        user_id: uuid.UUID,
        *,
        n: int = 1,
        amount: Decimal | None = None,
        action: str = "payment.initiated",
        when: datetime | None = None,
    ) -> None:
        when = when or datetime.now(UTC)
        for _ in range(n):
            self._activity_seq += 1
            self.activity.append(
                ActivityEventRow(
                    id=self._activity_seq,
                    user_id=user_id,
                    action=action,
                    amount=amount,
                    currency="KES" if amount is not None else None,
                    occurred_at=when,
                )
            )
