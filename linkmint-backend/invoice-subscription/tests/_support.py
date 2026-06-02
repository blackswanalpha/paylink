"""Shared test doubles + helpers (imported by conftest and the integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``, the way identity-service would) and its PUBLIC PEM is injected via settings, so the
verifier-only JWT seam runs against REAL primitives — no issuer code here (this service is a
consumer). ``FakeInvoiceRepository`` is an in-memory mirror of :class:`InvoiceRepository` with the
same surface, so the unit suite runs without Docker. ``FakePaylink`` stands in for paylink-service.
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
from app.db.models import InvoiceLineRow, InvoiceRow
from app.paylinks.client import PaylinkError

# One ephemeral RSA keypair for the whole test run. The private key stands in for identity-service's
# signer; the public PEM is what invoice-subscription verifies with.
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
ADDR = "0x" + "a" * 40


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
        # Background tasks off by default so the TestClient lifespan never touches a real DB/broker.
        "overdue_sweep_enabled": False,
        "event_consumer_enabled": False,
    }
    base.update(overrides)
    return Settings(**base)


def mint_token(
    *,
    user_id: str | None = None,
    user_roles: list[str] | None = None,
    kyc_tier: int = 1,
    issuer: str = ISSUER,
    audience: str = AUDIENCE,
    private_pem: str | None = None,
    algorithm: str = "RS256",
    expired: bool = False,
) -> str:
    """Mint an RS256 access token the way identity-service would (for the verifier)."""
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
        "sid": "s1",
        "user_roles": user_roles if user_roles is not None else ["merchant"],
        "kyc_tier": kyc_tier,
    }
    key = private_pem if private_pem is not None else TEST_PRIVATE_PEM
    return pyjwt.encode(payload, key, algorithm=algorithm)


def merchant_headers(user_id: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {mint_token(user_id=user_id)}"}


class FakePaylink:
    """In-memory paylink-service stand-in. Configure to fail or omit pl_id."""

    def __init__(
        self, pl_id: str = "PLK_test", *, fail: bool = False, no_plid: bool = False
    ) -> None:
        self.pl_id = pl_id
        self.fail = fail
        self.no_plid = no_plid
        self.calls: list[dict[str, Any]] = []

    async def create(
        self,
        *,
        receiver: str,
        amount: int,
        currency: str,
        expiry: datetime,
        usage: str = "single",
        metadata: dict[str, Any] | None = None,
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        self.calls.append(
            {
                "receiver": receiver,
                "amount": amount,
                "currency": currency,
                "expiry": expiry,
                "usage": usage,
                "metadata": metadata,
                "idempotency_key": idempotency_key,
            }
        )
        if self.fail:
            raise PaylinkError("paylinks unavailable (test)")
        if self.no_plid:
            return {"status": "PENDING"}
        return {"pl_id": self.pl_id, "status": "PENDING"}


class FakeInvoiceRepository:
    """In-memory InvoiceRepository with matching method names/semantics."""

    def __init__(self) -> None:
        self.invoices: dict[uuid.UUID, InvoiceRow] = {}
        self.lines: list[InvoiceLineRow] = []
        self.events: list[tuple[uuid.UUID, str, dict[str, Any]]] = []
        self._line_seq = 0

    async def insert_invoice(self, row: InvoiceRow) -> InvoiceRow:
        now = datetime.now(UTC)
        if row.created_at is None:
            row.created_at = now
        if row.updated_at is None:
            row.updated_at = now
        self.invoices[row.invoice_id] = row
        return row

    async def get_invoice(self, invoice_id: uuid.UUID) -> InvoiceRow | None:
        return self.invoices.get(invoice_id)

    async def get_by_plid(self, pl_id: str) -> InvoiceRow | None:
        return next((r for r in self.invoices.values() if r.pl_id == pl_id), None)

    async def list_invoices(
        self,
        merchant_id: uuid.UUID,
        *,
        status: str | None = None,
        limit: int = 50,
        offset: int = 0,
    ) -> list[InvoiceRow]:
        rows = [
            r
            for r in self.invoices.values()
            if r.merchant_id == merchant_id and (status is None or r.status == status)
        ]
        rows.sort(key=lambda r: r.created_at, reverse=True)
        return rows[offset : offset + limit]

    async def find_overdue(self, now: datetime, *, limit: int = 100) -> list[InvoiceRow]:
        rows = [r for r in self.invoices.values() if r.status == "OPEN" and r.due_at < now]
        rows.sort(key=lambda r: r.due_at)
        return rows[:limit]

    async def insert_lines(self, rows: list[InvoiceLineRow]) -> list[InvoiceLineRow]:
        for r in rows:
            self._line_seq += 1
            r.id = self._line_seq
            self.lines.append(r)
        return rows

    async def list_lines(self, invoice_id: uuid.UUID) -> list[InvoiceLineRow]:
        return [r for r in self.lines if r.invoice_id == invoice_id]

    async def add_event(self, invoice_id: uuid.UUID, kind: str, payload: dict[str, Any]) -> None:
        self.events.append((invoice_id, kind, payload))

    # ── test helper ──
    def seed(
        self,
        *,
        merchant_id: uuid.UUID,
        status: str,
        pl_id: str | None = None,
        due_at: datetime | None = None,
        total: int = 1000,
        currency: str = "PLN",
        payee_addr: str = ADDR,
        customer_id: uuid.UUID | None = None,
    ) -> InvoiceRow:
        iid = uuid.uuid4()
        now = datetime.now(UTC)
        row = InvoiceRow(
            invoice_id=iid,
            merchant_id=merchant_id,
            customer_id=customer_id,
            payee_addr=payee_addr,
            pl_id=pl_id,
            currency=currency,
            subtotal=Decimal(total),
            tax=Decimal(0),
            total=Decimal(total),
            status=status,
            due_at=due_at or (now + timedelta(days=7)),
            paid_at=None,
            created_at=now,
            updated_at=now,
        )
        self.invoices[iid] = row
        return row


def create_body(**overrides: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "payee_addr": ADDR,
        "currency": "PLN",
        "due_at": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
        "lines": [{"description": "Item", "quantity": "2", "unit_price": 1500, "tax_rate": "0.16"}],
    }
    body.update(overrides)
    return body
