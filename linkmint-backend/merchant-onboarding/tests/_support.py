"""Shared test doubles + helpers (imported by conftest and integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``) and its PUBLIC PEM is injected via settings, so merchant-onboarding's verifier-only
JWT seam, RBAC, AES-GCM bank cipher, and idempotency are all exercised with REAL primitives — no
issuer code here (merchant-onboarding is a consumer). The fake repository is an in-memory mirror of
:class:`MerchantRepository` with the same surface, so the unit/API suite runs without Docker.
"""

from __future__ import annotations

import base64
import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.config import Settings
from app.db.models import (
    BankAccountRow,
    ContractRow,
    DocumentRow,
    MerchantRow,
)
from app.storage.object_store import ObjectStore

# One ephemeral RSA keypair for the whole test run (avoids per-test keygen cost). The private key
# stands in for identity-service's signer; the public PEM is what merchant-onboarding verifies with.
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

# Fixed 32-byte bank-ref encryption key (deterministic).
TEST_BANK_KEY = base64.b64encode(bytes(range(32))).decode()

ISSUER = "linkmint-identity"
AUDIENCE = "linkmint"


async def noop_commit() -> None:
    return None


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "jwt_public_key_pem": TEST_PUBLIC_PEM,
        "jwt_issuer": ISSUER,
        "jwt_audience": AUDIENCE,
        "bank_encryption_key": TEST_BANK_KEY,
        "allowed_countries": "KE",
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
    """Mint an RS256 access token the way identity-service would (for the merchant verifier)."""
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


def auth_headers_for(
    org_id: str, *, role: str = "owner", user_id: str | None = None
) -> dict[str, str]:
    """Bearer header for a principal who is a member of ``org_id`` with ``role`` (shared helper)."""
    token = mint_token(user_id=user_id, roles=[(org_id, role)])
    return {"Authorization": f"Bearer {token}"}


class FakeObjectStore(ObjectStore):
    """In-memory object store for unit tests."""

    def __init__(self) -> None:
        self.objects: dict[str, bytes] = {}

    def put(self, key: str, data: bytes, *, content_type: str | None = None) -> None:
        self.objects[key] = data

    def get(self, key: str) -> bytes:
        return self.objects[key]


class FakeRepository:
    """In-memory MerchantRepository with matching method names/semantics."""

    def __init__(self) -> None:
        self.merchants: dict[uuid.UUID, MerchantRow] = {}
        self.bank_accounts: dict[uuid.UUID, BankAccountRow] = {}
        self.documents: dict[uuid.UUID, DocumentRow] = {}
        self.contracts: dict[int, ContractRow] = {}
        self._contract_seq = 0
        self.events: list[tuple[str, uuid.UUID | None, str, dict[str, Any]]] = []

    # ── merchants ──
    async def insert_merchant(self, row: MerchantRow) -> MerchantRow:
        if row.fee_tier is None:
            row.fee_tier = "standard"
        self.merchants[row.merchant_id] = row
        return row

    async def get_merchant(self, merchant_id: uuid.UUID) -> MerchantRow | None:
        return self.merchants.get(merchant_id)

    async def get_merchant_by_org(self, org_id: uuid.UUID) -> MerchantRow | None:
        return next((m for m in self.merchants.values() if m.org_id == org_id), None)

    # ── bank accounts ──
    async def insert_bank_account(self, row: BankAccountRow) -> BankAccountRow:
        self.bank_accounts[row.bank_account_id] = row
        return row

    async def get_bank_account(self, bank_account_id: uuid.UUID) -> BankAccountRow | None:
        return self.bank_accounts.get(bank_account_id)

    async def list_bank_accounts(self, merchant_id: uuid.UUID) -> list[BankAccountRow]:
        return [b for b in self.bank_accounts.values() if b.merchant_id == merchant_id]

    async def count_verified_bank_accounts(self, merchant_id: uuid.UUID) -> int:
        return sum(
            1
            for b in self.bank_accounts.values()
            if b.merchant_id == merchant_id and b.status == "VERIFIED"
        )

    # ── documents ──
    async def insert_document(self, row: DocumentRow) -> DocumentRow:
        if row.uploaded_at is None:
            row.uploaded_at = datetime.now(UTC)
        self.documents[row.document_id] = row
        return row

    async def list_documents(self, merchant_id: uuid.UUID) -> list[DocumentRow]:
        return [d for d in self.documents.values() if d.merchant_id == merchant_id]

    # ── contracts ──
    async def insert_contract(self, row: ContractRow) -> ContractRow:
        self._contract_seq += 1
        row.id = self._contract_seq
        if row.accepted_at is None:
            row.accepted_at = datetime.now(UTC)
        self.contracts[row.id] = row
        return row

    async def list_contracts(self, merchant_id: uuid.UUID) -> list[ContractRow]:
        rows = [c for c in self.contracts.values() if c.merchant_id == merchant_id]
        return sorted(rows, key=lambda c: c.accepted_at, reverse=True)

    async def count_contracts(self, merchant_id: uuid.UUID) -> int:
        return sum(1 for c in self.contracts.values() if c.merchant_id == merchant_id)

    # ── events ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self.events.append((subject_type, subject_id, kind, payload))
