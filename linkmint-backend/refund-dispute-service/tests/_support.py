"""Shared test doubles + helpers (imported by conftest and the integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``, the way identity-service would) and its PUBLIC PEM is injected via settings, so the
verifier-only JWT seam runs against REAL primitives. ``FakeRefundRepository`` is an in-memory mirror
of :class:`RefundRepository` with the same surface, so the unit suite runs without Docker. The fake
clients stand in for payment-orchestrator / paylink-service.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from typing import Any

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.config import Settings
from app.db.models import DisputeEvidenceRow, DisputeRow, RefundRow, VerifiedPaylinkRow
from app.paylinks.client import PaylinkAmount
from app.payments.client import PaymentInfo
from app.security.hmac import compute_signature

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
WEBHOOK_SECRET = "devnet-dispute-secret"


async def noop_commit() -> None:
    return None


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "jwt_public_key_pem": TEST_PUBLIC_PEM,
        "jwt_issuer": ISSUER,
        "jwt_audience": AUDIENCE,
        "event_publisher_mode": "noop",
        # Background tasks off so the TestClient lifespan never touches a real DB/broker.
        "sweep_enabled": False,
        "event_consumer_enabled": False,
        "amount_validation": "lenient",
        "clawback_mode": "event",
        "default_currency": "KES",
        "admin_user_roles": "admin",
        "webhook_secrets": f"stub:{WEBHOOK_SECRET}",
    }
    base.update(overrides)
    return Settings(**base)


def mint_token(
    *,
    user_id: str | None = None,
    roles: list[dict[str, str]] | None = None,
    user_roles: list[str] | None = None,
    kyc_tier: int = 1,
    issuer: str = ISSUER,
    audience: str = AUDIENCE,
    private_pem: str | None = None,
    algorithm: str = "RS256",
    expired: bool = False,
) -> str:
    """Mint an RS256 access token the way identity-service would (for the verifier)."""
    now = datetime.now(UTC) - timedelta(hours=2) if expired else datetime.now(UTC)
    exp = now + timedelta(hours=1)
    payload = {
        "sub": user_id or str(uuid.uuid4()),
        "iss": issuer,
        "aud": audience,
        "iat": int(now.timestamp()),
        "nbf": int(now.timestamp()),
        "exp": int(exp.timestamp()),
        "jti": uuid.uuid4().hex,
        "sid": "s1",
        "roles": roles if roles is not None else [],
        "user_roles": user_roles if user_roles is not None else ["merchant"],
        "kyc_tier": kyc_tier,
    }
    key = private_pem if private_pem is not None else TEST_PRIVATE_PEM
    return pyjwt.encode(payload, key, algorithm=algorithm)


def auth_headers(
    user_id: str | None = None,
    *,
    roles: list[dict[str, str]] | None = None,
    user_roles: list[str] | None = None,
) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {mint_token(user_id=user_id, roles=roles, user_roles=user_roles)}"
    }


def sign(secret: str, raw: bytes) -> dict[str, str]:
    return {"X-Signature": compute_signature(secret, raw)}


class FakePaymentsClient:
    """Stand-in for payment-orchestrator. Configure with a mapping of payment_id → PaymentInfo."""

    def __init__(self, payments: dict[str, PaymentInfo] | None = None) -> None:
        self.payments = payments or {}

    def add(
        self,
        payment_id: str,
        *,
        paylink_id: str = "0xpl",
        rail: str = "mpesa",
        status: str = "SETTLED",
    ) -> PaymentInfo:
        info = PaymentInfo(id=payment_id, paylink_id=paylink_id, rail=rail, status=status)
        self.payments[payment_id] = info
        return info

    async def get(self, payment_id: str) -> PaymentInfo | None:
        return self.payments.get(payment_id)


class FakePaylinksClient:
    """Stand-in for paylink-service. Returns the configured original amount (or None)."""

    def __init__(self, amounts: dict[str, PaylinkAmount] | None = None) -> None:
        self.amounts = amounts or {}

    def add(self, paylink_id: str, amount_minor: int, currency: str = "KES") -> None:
        self.amounts[paylink_id] = PaylinkAmount(amount_minor=amount_minor, currency=currency)

    async def get_amount(self, paylink_id: str) -> PaylinkAmount | None:
        return self.amounts.get(paylink_id)


_ACTIVE = ("REQUESTED", "APPROVED", "PROCESSING", "COMPLETED")


class FakeRefundRepository:
    """In-memory RefundRepository with matching method names/semantics."""

    def __init__(self) -> None:
        self.refunds: dict[uuid.UUID, RefundRow] = {}
        self.disputes: dict[uuid.UUID, DisputeRow] = {}
        self.evidence: list[DisputeEvidenceRow] = []
        self.verified: dict[str, VerifiedPaylinkRow] = {}
        self.events: list[tuple[str, str, dict[str, Any]]] = []

    @staticmethod
    def _stamp(row: Any) -> None:
        now = datetime.now(UTC)
        if getattr(row, "created_at", None) is None:
            row.created_at = now
        if hasattr(row, "updated_at") and getattr(row, "updated_at", None) is None:
            row.updated_at = now

    # ── refunds ──
    async def insert_refund(self, row: RefundRow) -> RefundRow:
        self._stamp(row)
        self.refunds[row.refund_id] = row
        return row

    async def get_refund(self, refund_id: uuid.UUID) -> RefundRow | None:
        return self.refunds.get(refund_id)

    async def list_refunds_by_payment(self, payment_id: str) -> list[RefundRow]:
        rows = [r for r in self.refunds.values() if r.payment_id == payment_id]
        return sorted(rows, key=lambda r: r.created_at)

    async def active_refund_total(self, payment_id: str) -> int:
        return sum(
            int(r.amount_minor)
            for r in self.refunds.values()
            if r.payment_id == payment_id and r.state in _ACTIVE
        )

    async def list_processing_refunds_before(self, cutoff: datetime) -> list[RefundRow]:
        return [
            r for r in self.refunds.values() if r.state == "PROCESSING" and r.updated_at < cutoff
        ]

    # ── disputes ──
    async def insert_dispute_if_absent(self, row: DisputeRow) -> bool:
        for d in self.disputes.values():
            if d.provider == row.provider and d.provider_dispute_id == row.provider_dispute_id:
                return False
        self._stamp(row)
        self.disputes[row.dispute_id] = row
        return True

    async def get_dispute(self, dispute_id: uuid.UUID) -> DisputeRow | None:
        return self.disputes.get(dispute_id)

    async def get_dispute_by_provider_ref(
        self, provider: str, provider_dispute_id: str
    ) -> DisputeRow | None:
        for d in self.disputes.values():
            if d.provider == provider and d.provider_dispute_id == provider_dispute_id:
                return d
        return None

    async def list_open_disputes_due_before(self, cutoff: datetime) -> list[DisputeRow]:
        return [
            d for d in self.disputes.values() if d.state == "OPEN" and d.evidence_due_at < cutoff
        ]

    # ── evidence ──
    async def insert_evidence(self, row: DisputeEvidenceRow) -> DisputeEvidenceRow:
        self._stamp(row)
        self.evidence.append(row)
        return row

    async def list_evidence(self, dispute_id: uuid.UUID) -> list[DisputeEvidenceRow]:
        rows = [e for e in self.evidence if e.dispute_id == dispute_id]
        return sorted(rows, key=lambda e: e.created_at)

    async def count_evidence(self, dispute_id: uuid.UUID) -> int:
        return len([e for e in self.evidence if e.dispute_id == dispute_id])

    # ── verified-paylink projection ──
    async def upsert_verified_paylink(
        self,
        *,
        paylink_id: str,
        tx_hash: str | None,
        block_height: int | None,
        amount_minor: int | None,
        currency: str | None,
        verified_at: datetime,
        payload: dict[str, Any],
    ) -> None:
        existing = self.verified.get(paylink_id)
        amt = Decimal(amount_minor) if amount_minor is not None else None
        if existing is not None:
            existing.tx_hash = tx_hash
            existing.block_height = block_height
            if amt is not None:
                existing.amount_minor = amt
            if currency is not None:
                existing.currency = currency
            existing.verified_at = verified_at
            existing.payload = payload
        else:
            self.verified[paylink_id] = VerifiedPaylinkRow(
                paylink_id=paylink_id,
                tx_hash=tx_hash,
                block_height=block_height,
                amount_minor=amt,
                currency=currency,
                verified_at=verified_at,
                payload=payload,
            )

    async def get_verified_paylink(self, paylink_id: str) -> VerifiedPaylinkRow | None:
        return self.verified.get(paylink_id)

    def seed_verified(self, paylink_id: str, amount_minor: int, currency: str = "KES") -> None:
        self.verified[paylink_id] = VerifiedPaylinkRow(
            paylink_id=paylink_id,
            tx_hash="0xtx",
            block_height=1,
            amount_minor=Decimal(amount_minor),
            currency=currency,
            verified_at=datetime.now(UTC),
            payload={},
        )

    # ── events (outbox) ──
    async def add_event(self, entity_id: str, kind: str, payload: dict[str, Any]) -> None:
        self.events.append((entity_id, kind, payload))

    # ── test helpers ──
    def event_kinds(self) -> list[str]:
        return [k for (_e, k, _p) in self.events]

    def event_payload(self, kind: str) -> dict[str, Any]:
        for _e, k, p in self.events:
            if k == kind:
                return p
        raise AssertionError(f"event {kind} not emitted; have {self.event_kinds()}")
