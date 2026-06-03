"""Shared test doubles + helpers (imported by conftest and the integration tests).

A single RSA keypair is generated once (module load): its PRIVATE key signs RS256 test tokens
(``mint_token``, the way identity-service would) and its PUBLIC PEM is injected via settings, so the
verifier-only JWT seam runs against REAL primitives. ``FakePricingRepository`` is an in-memory
mirror of :class:`PricingRepository` with the same surface (seeded with the migration's default
tiers/rails), so the unit suite runs without Docker. ``FakeFxProvider`` stands in for the FX source.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any

import jwt as pyjwt
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.config import Settings
from app.db.models import (
    FxRateRow,
    MerchantPricingRow,
    PlatformFeeAccrualRow,
    PlatformFeeInvoiceRow,
    QuoteRow,
    RailFeeRow,
    TierRow,
)
from app.fx.provider import Rate

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
INTERNAL_TOKEN = "test-internal-token"

# Default seeds (mirror alembic 0001): tier name → platform_pct_bps; rail → (pct_bps, fixed).
SEED_TIERS = {
    "standard": ("Standard", 250),
    "startup": ("Startup", 150),
    "growth": ("Growth", 200),
    "scale": ("Scale", 120),
    "enterprise": ("Enterprise", 80),
}
SEED_RAILS = {"mpesa": (150, 0), "card": (290, 30), "bank": (50, 0), "crypto": (100, 0)}


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
        "invoice_sweep_enabled": False,
        "event_consumer_enabled": False,
        "default_currency": "KES",
        "admin_user_roles": "admin",
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
    now = datetime.now(UTC)
    if expired:
        from datetime import timedelta

        now = now - timedelta(hours=2)
    from datetime import timedelta

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


class FakeFxProvider:
    """In-memory FX source. Counts ``get_rate`` calls so cache-hit tests can assert no fetch."""

    def __init__(self, rates: dict[tuple[str, str], Decimal] | None = None) -> None:
        self.rates = rates or {
            ("USD", "KES"): Decimal("129.50"),
            ("EUR", "KES"): Decimal("140.00"),
        }
        self.calls = 0

    async def get_rate(self, base: str, quote: str) -> Rate | None:
        base, quote = base.upper(), quote.upper()
        if base == quote:
            return Rate(base, quote, Decimal(1), "identity", datetime.now(UTC))
        self.calls += 1
        rate = self.rates.get((base, quote))
        if rate is None:
            return None
        return Rate(base, quote, rate, "static", datetime.now(UTC))


class FakePricingRepository:
    """In-memory PricingRepository with matching method names/semantics."""

    def __init__(self, *, seed: bool = True) -> None:
        self.tiers: dict[str, TierRow] = {}
        self.rails: list[RailFeeRow] = []
        self.merchant_pricing: dict[uuid.UUID, MerchantPricingRow] = {}
        self.quotes: list[QuoteRow] = []
        self.fx_rates: list[FxRateRow] = []
        self.accruals: list[PlatformFeeAccrualRow] = []
        self.invoices: dict[uuid.UUID, PlatformFeeInvoiceRow] = {}
        self.events: list[tuple[str, str, dict[str, Any]]] = []
        self._accrual_seq = 0
        if seed:
            self.seed_defaults()

    def seed_defaults(self) -> None:
        for tier, (name, bps) in SEED_TIERS.items():
            self.tiers[tier] = TierRow(
                tier=tier,
                display_name=name,
                platform_pct_bps=bps,
                platform_fixed=Decimal(0),
                fixed_currency="KES",
                active=True,
            )
        for rid, (rail, (bps, fixed)) in enumerate(SEED_RAILS.items(), start=1):
            self.rails.append(
                RailFeeRow(
                    id=rid,
                    rail=rail,
                    tier=None,
                    pct_bps=bps,
                    fixed=Decimal(fixed),
                    fixed_currency="KES",
                    active=True,
                )
            )

    async def get_tier(self, tier: str) -> TierRow | None:
        return self.tiers.get(tier)

    async def list_tiers(self, *, active_only: bool = False) -> list[TierRow]:
        rows = [r for r in self.tiers.values() if r.active or not active_only]
        return sorted(rows, key=lambda r: r.platform_pct_bps, reverse=True)

    async def get_rail_fee(self, rail: str, tier: str) -> RailFeeRow | None:
        specific = [r for r in self.rails if r.rail == rail and r.active and r.tier == tier]
        if specific:
            return specific[0]
        glob = [r for r in self.rails if r.rail == rail and r.active and r.tier is None]
        return glob[0] if glob else None

    async def list_rail_fees(self, *, active_only: bool = True) -> list[RailFeeRow]:
        return [r for r in self.rails if r.active or not active_only]

    async def get_merchant_pricing(self, merchant_id: uuid.UUID) -> MerchantPricingRow | None:
        return self.merchant_pricing.get(merchant_id)

    async def upsert_merchant_pricing(
        self,
        merchant_id: uuid.UUID,
        *,
        tier: str,
        source: str,
        org_id: uuid.UUID | None = None,
    ) -> None:
        now = datetime.now(UTC)
        existing = self.merchant_pricing.get(merchant_id)
        if existing is not None:
            existing.tier = tier
            existing.source = source
            existing.effective_at = now
            existing.updated_at = now
            if org_id is not None:  # coalesce — never clear a captured org_id
                existing.org_id = org_id
        else:
            self.merchant_pricing[merchant_id] = MerchantPricingRow(
                merchant_id=merchant_id,
                org_id=org_id,
                tier=tier,
                source=source,
                effective_at=now,
                updated_at=now,
            )

    async def insert_quote(self, row: QuoteRow) -> QuoteRow:
        self.quotes.append(row)
        return row

    async def insert_fx_rate(self, row: FxRateRow) -> FxRateRow:
        self.fx_rates.append(row)
        return row

    async def latest_fx_rate(self, base: str, quote: str) -> FxRateRow | None:
        matches = [
            r for r in self.fx_rates if r.base_currency == base and r.quote_currency == quote
        ]
        return matches[-1] if matches else None

    async def insert_accrual(
        self,
        *,
        merchant_id: uuid.UUID,
        period: str,
        amount: Decimal,
        currency: str,
        source_ref: str,
        occurred_at: datetime,
        quote_id: uuid.UUID | None = None,
    ) -> tuple[bool, int]:
        for a in self.accruals:
            if a.merchant_id == merchant_id and a.source_ref == source_ref:
                return False, a.id
        self._accrual_seq += 1
        row = PlatformFeeAccrualRow(
            id=self._accrual_seq,
            merchant_id=merchant_id,
            period=period,
            amount=amount,
            currency=currency,
            source_ref=source_ref,
            occurred_at=occurred_at,
            quote_id=quote_id,
            invoice_id=None,
        )
        self.accruals.append(row)
        return True, row.id

    async def unbilled_accruals(
        self, period: str, *, merchant_id: uuid.UUID | None = None
    ) -> list[PlatformFeeAccrualRow]:
        rows = [
            a
            for a in self.accruals
            if a.period == period
            and a.invoice_id is None
            and (merchant_id is None or a.merchant_id == merchant_id)
        ]
        return sorted(rows, key=lambda a: (str(a.merchant_id), a.id))

    async def mark_accruals_invoiced(self, ids: list[int], invoice_id: uuid.UUID) -> None:
        for a in self.accruals:
            if a.id in ids:
                a.invoice_id = invoice_id

    async def get_invoice_for_period(
        self, merchant_id: uuid.UUID, period: str
    ) -> PlatformFeeInvoiceRow | None:
        for inv in self.invoices.values():
            if inv.merchant_id == merchant_id and inv.period == period:
                return inv
        return None

    async def insert_invoice(self, row: PlatformFeeInvoiceRow) -> PlatformFeeInvoiceRow:
        self.invoices[row.invoice_id] = row
        return row

    async def add_event(self, entity_id: str, kind: str, payload: dict[str, Any]) -> None:
        self.events.append((entity_id, kind, payload))

    # ── test helpers ──
    def event_kinds(self) -> list[str]:
        return [k for (_e, k, _p) in self.events]

    def seed_merchant(
        self,
        merchant_id: uuid.UUID,
        *,
        tier: str = "standard",
        org_id: uuid.UUID | None = None,
    ) -> MerchantPricingRow:
        now = datetime.now(UTC)
        row = MerchantPricingRow(
            merchant_id=merchant_id,
            org_id=org_id,
            tier=tier,
            source="manual",
            effective_at=now,
            updated_at=now,
        )
        self.merchant_pricing[merchant_id] = row
        return row


def quote_body(**overrides: Any) -> dict[str, Any]:
    body: dict[str, Any] = {
        "merchant_id": str(uuid.uuid4()),
        "gross": 100000,
        "currency": "KES",
        "rails": ["mpesa"],
        "tiers": ["standard"],
    }
    body.update(overrides)
    return body
