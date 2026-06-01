"""Shared test doubles + helpers (imported by conftest and the integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``, the way identity-service would) and its PUBLIC PEM is injected via settings, so the
verifier-only JWT seam, the admin/MFA gate, and the default-deny scope model are all exercised with
REAL primitives — no issuer code here (admin-backoffice is a consumer). Providers, the staff repo,
and the audit sink are in-memory fakes so the unit/API suite runs without Docker or the upstreams.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Any

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.audit.sink import AuditRecord
from app.config import Settings
from app.providers.fake import FakeProvider
from app.providers.registry import ProviderRegistry
from app.security.jwt import AccessClaims, OrgRole

# One ephemeral RSA keypair for the whole test run. The private key stands in for identity-service's
# signer; the public PEM is what admin-backoffice verifies with.
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

ISSUER = "linkmint-identity"
AUDIENCE = "linkmint"

# Sample entity ids seeded into the fake providers (referenced by the API/integration tests).
SAMPLE_USER_ID = "11111111-1111-1111-1111-111111111111"
SAMPLE_MERCHANT_ID = "22222222-2222-2222-2222-222222222222"
SAMPLE_PAYLINK_ID = "0x" + "ab" * 32  # 66-char 0x hash
SAMPLE_PAYMENT_ID = "33333333-3333-3333-3333-333333333333"


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "jwt_public_key_pem": TEST_PUBLIC_PEM,
        "jwt_issuer": ISSUER,
        "jwt_audience": AUDIENCE,
        "audit_sink_mode": "noop",
    }
    base.update(overrides)
    return Settings(**base)


def _role_claim(triple: tuple[str, ...]) -> dict[str, str]:
    claim = {"org_id": triple[0], "role": triple[1]}
    if len(triple) > 2 and triple[2]:
        claim["type"] = triple[2]
    return claim


def mint_token(
    *,
    user_id: str | None = None,
    roles: list[tuple[str, ...]] | None = None,
    mfa: bool = False,
    user_roles: list[str] | None = None,
    kyc_tier: int = 1,
    sid: str = "s1",
    issuer: str = ISSUER,
    audience: str = AUDIENCE,
    private_pem: str | None = None,
    algorithm: str = "RS256",
    expired: bool = False,
) -> str:
    """Mint an RS256 access token the way identity-service would (roles are (org_id, role[, type)))."""
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
        "roles": [_role_claim(r) for r in (roles or [])],
        "user_roles": user_roles if user_roles is not None else ["payer"],
        "kyc_tier": kyc_tier,
        "mfa": mfa,
        "amr": ["pwd", "mfa"] if mfa else ["pwd"],
    }
    key = private_pem if private_pem is not None else TEST_PRIVATE_PEM
    return pyjwt.encode(payload, key, algorithm=algorithm)


def principal_admin(sub: str = SAMPLE_USER_ID) -> AccessClaims:
    """An MFA-elevated admin principal (for direct service-level tests)."""
    return AccessClaims(
        user_id=sub,
        roles=[OrgRole("o1", "admin", "admin")],
        user_roles=["payer"],
        kyc_tier=0,
        jti="j",
        sid="s",
        mfa=True,
        amr=["pwd", "mfa"],
    )


@dataclass
class _Staff:
    scopes: list[str]


class FakeAdminRepository:
    """In-memory AdminRepository (the default-deny staff grant store)."""

    def __init__(self) -> None:
        self._staff: dict[str, list[str]] = {}

    def grant(self, sub: str, scopes: list[str]) -> None:
        self._staff[sub] = list(scopes)

    async def get_staff(self, sub: str) -> _Staff | None:
        scopes = self._staff.get(sub)
        return None if scopes is None else _Staff(scopes=list(scopes))


class CapturingAuditSink:
    """Records every emitted AuditRecord so tests can assert audit-by-construction."""

    def __init__(self) -> None:
        self.records: list[AuditRecord] = []

    async def emit(self, record: AuditRecord) -> None:
        self.records.append(record)


def fake_registry(*, fail: set[str] | None = None) -> ProviderRegistry:
    """A registry of in-memory providers seeded with one entity each (mark types in ``fail`` down)."""
    fail = fail or set()
    seeds: dict[str, dict[str, dict[str, Any]]] = {
        "user": {
            SAMPLE_USER_ID: {
                "user_id": SAMPLE_USER_ID,
                "email": "alice@example.com",
                "status": "ACTIVE",
                "label": "alice@example.com",
            }
        },
        "merchant": {
            SAMPLE_MERCHANT_ID: {
                "merchant_id": SAMPLE_MERCHANT_ID,
                "business_name": "Acme Ltd",
                "status": "ACTIVE",
                "label": "Acme Ltd",
            }
        },
        "paylink": {
            SAMPLE_PAYLINK_ID: {
                "pl_id": SAMPLE_PAYLINK_ID,
                "status": "CREATED",
                "label": SAMPLE_PAYLINK_ID,
            }
        },
        "payment": {
            SAMPLE_PAYMENT_ID: {
                "id": SAMPLE_PAYMENT_ID,
                "status": "SETTLED",
                "label": SAMPLE_PAYMENT_ID,
            }
        },
    }
    return ProviderRegistry([FakeProvider(t, entities=seeds[t], fail=(t in fail)) for t in seeds])


def staff_headers(
    admin_repo: FakeAdminRepository,
    *,
    scopes: tuple[str, ...] = ("support.read",),
    mfa: bool = True,
    admin: bool = True,
    sub: str | None = None,
) -> dict[str, str]:
    """Bearer header for a staff member, seeding their grant into the fake repo."""
    sub = sub or str(uuid.uuid4())
    if scopes:
        admin_repo.grant(sub, list(scopes))
    role = ("org-1", "admin", "admin") if admin else ("org-1", "owner", "merchant")
    token = mint_token(user_id=sub, roles=[role], mfa=mfa)
    return {"Authorization": f"Bearer {token}"}
