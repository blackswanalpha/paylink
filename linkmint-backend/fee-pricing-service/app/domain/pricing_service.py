"""Pricing domain service: tiers, merchant-pricing, and quoting.

The HTTP path (deps.get_services), the bus consumer, and the sweeper all build the same Services
bundle over a fresh session, so the rules live in one place. ``quote`` resolves + LOCKS the FX rate
(via FxService), computes one result per (tier, rail), persists each quote with its locked rate for
audit, and emits ``pricing.fee_quote.issued``. Non-custodial (A.1): only metadata is stored.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import Decimal

from app.config import Settings
from app.db.models import QuoteRow
from app.db.repositories import PricingRepository
from app.domain.fx_service import FxService
from app.domain.models import (
    DEFAULT_TIER,
    Rail,
    RailFee,
    TierFee,
    compute_quote,
)
from app.errors import AppError, ErrorCode
from app.events.publisher import PRICING_FEE_QUOTE_ISSUED, Publisher
from app.fx.provider import Rate
from app.logging import get_logger

log = get_logger("pricing.service")

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class RailFeeView:
    rail: str
    pct_bps: int
    fixed: int
    fixed_currency: str


@dataclass(frozen=True)
class MerchantPricingView:
    merchant_id: str
    org_id: str | None
    tier: str
    display_name: str
    platform_pct_bps: int
    platform_fixed: int
    fixed_currency: str
    rail_fees: list[RailFeeView]
    effective_at: datetime


@dataclass(frozen=True)
class QuoteIssued:
    quote_id: uuid.UUID
    tier: str
    rail: str
    breakdown: dict[str, object]
    gross: int
    currency: str
    gross_settled: int
    settle_currency: str
    platform_fee: int
    rail_fee: int
    net: int
    fx: Rate | None


class PricingService:
    def __init__(
        self,
        repo: PricingRepository,
        commit: _Commit,
        publisher: Publisher,
        fx: FxService,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._fx = fx
        self._settings = settings

    # ── tiers ──
    async def list_tiers(self, *, active_only: bool = False) -> list:
        return await self._repo.list_tiers(active_only=active_only)

    async def _tier_fee(self, tier: str) -> TierFee:
        row = await self._repo.get_tier(tier)
        if row is None or not row.active:
            raise AppError(
                ErrorCode.TIER_NOT_FOUND, "unknown or inactive tier", details={"tier": tier}
            )
        return TierFee(
            tier=row.tier,
            platform_pct_bps=row.platform_pct_bps,
            platform_fixed=int(row.platform_fixed),
            fixed_currency=row.fixed_currency,
        )

    async def _rail_fee(self, rail: str, tier: str) -> RailFee:
        row = await self._repo.get_rail_fee(rail, tier)
        if row is None:
            raise AppError(
                ErrorCode.RAIL_NOT_FOUND, "no fee schedule for rail", details={"rail": rail}
            )
        return RailFee(
            rail=row.rail,
            pct_bps=row.pct_bps,
            fixed=int(row.fixed),
            fixed_currency=row.fixed_currency,
        )

    # ── merchant pricing (read) ──
    async def get_merchant_pricing_view(self, merchant_id: uuid.UUID) -> MerchantPricingView:
        mp = await self._repo.get_merchant_pricing(merchant_id)
        if mp is None:
            raise AppError(
                ErrorCode.MERCHANT_PRICING_NOT_FOUND,
                "no pricing configured for merchant",
                details={"merchant_id": str(merchant_id)},
            )
        tier_row = await self._repo.get_tier(mp.tier)
        display_name = tier_row.display_name if tier_row else mp.tier
        platform_pct_bps = tier_row.platform_pct_bps if tier_row else 0
        platform_fixed = int(tier_row.platform_fixed) if tier_row else 0
        fixed_currency = tier_row.fixed_currency if tier_row else self._settings.default_currency
        rail_fees: list[RailFeeView] = []
        for rail in Rail:
            row = await self._repo.get_rail_fee(rail.value, mp.tier)
            if row is not None:
                rail_fees.append(
                    RailFeeView(
                        rail=row.rail,
                        pct_bps=row.pct_bps,
                        fixed=int(row.fixed),
                        fixed_currency=row.fixed_currency,
                    )
                )
        return MerchantPricingView(
            merchant_id=str(mp.merchant_id),
            org_id=str(mp.org_id) if mp.org_id else None,
            tier=mp.tier,
            display_name=display_name,
            platform_pct_bps=platform_pct_bps,
            platform_fixed=platform_fixed,
            fixed_currency=fixed_currency,
            rail_fees=rail_fees,
            effective_at=mp.effective_at,
        )

    async def _resolve_merchant_tier(self, merchant_id: uuid.UUID) -> str:
        mp = await self._repo.get_merchant_pricing(merchant_id)
        return mp.tier if mp is not None else DEFAULT_TIER

    # ── merchant pricing (consumer write) ──
    async def upsert_merchant_pricing(
        self, *, merchant_id: str, tier: str, source: str, org_id: str | None
    ) -> None:
        """Idempotent tier upsert from the bus. Resilient: a bad id is logged + skipped (never
        crashes the consumer); an unknown tier falls back to the default with a warning."""
        try:
            mid = uuid.UUID(merchant_id)
        except ValueError:
            log.warning("merchant_pricing_bad_id", merchant_id=merchant_id)
            return
        oid: uuid.UUID | None = None
        if org_id:
            try:
                oid = uuid.UUID(org_id)
            except ValueError:
                log.warning("merchant_pricing_bad_org_id", org_id=org_id)
        known = await self._repo.get_tier(tier)
        final_tier = tier if known is not None else DEFAULT_TIER
        if known is None:
            log.warning("tier_unknown", tier=tier, fallback=final_tier)
        await self._repo.upsert_merchant_pricing(mid, tier=final_tier, source=source, org_id=oid)
        await self._commit()

    # ── quoting ──
    async def quote(
        self,
        *,
        merchant_id: uuid.UUID,
        gross: int,
        currency: str,
        settle_currency: str,
        rails: list[str],
        tiers: list[str] | None,
    ) -> list[QuoteIssued]:
        currency = currency.upper()
        settle_currency = settle_currency.upper()

        # Resolve + lock the FX rate once (reused for every tier/rail pair in this request).
        fx: Rate | None = None
        if currency != settle_currency:
            fx = await self._fx.rate_for(currency, settle_currency)
        locked_at = datetime.now(UTC).isoformat()

        tier_names = tiers if tiers else [await self._resolve_merchant_tier(merchant_id)]

        issued: list[QuoteIssued] = []
        for tname in tier_names:
            tier_fee = await self._tier_fee(tname)
            for rname in rails:
                rail_fee = await self._rail_fee(rname, tname)
                qr = compute_quote(
                    gross_minor=gross,
                    currency=currency,
                    settle_currency=settle_currency,
                    tier=tier_fee,
                    rail=rail_fee,
                    fx=fx,
                    locked_at=locked_at,
                )
                quote_id = uuid.uuid4()
                row = QuoteRow(
                    quote_id=quote_id,
                    merchant_id=merchant_id,
                    tier=tname,
                    rail=rname,
                    gross=Decimal(gross),
                    currency=currency,
                    settle_currency=settle_currency,
                    platform_fee=Decimal(qr.platform_fee),
                    rail_fee=Decimal(qr.rail_fee),
                    net=Decimal(qr.net),
                    fx_base=fx.base if fx else None,
                    fx_quote=fx.quote if fx else None,
                    fx_rate=fx.rate if fx else None,
                    breakdown=qr.breakdown,
                )
                await self._repo.insert_quote(row)
                payload = {
                    "quote_id": str(quote_id),
                    "merchant_id": str(merchant_id),
                    "tier": tname,
                    "rail": rname,
                    "gross": qr.gross,
                    "currency": currency,
                    "gross_settled": qr.gross_settled,
                    "settle_currency": settle_currency,
                    "platform_fee": qr.platform_fee,
                    "rail_fee": qr.rail_fee,
                    "net": qr.net,
                    "fx": qr.breakdown["fx"],
                }
                await self._repo.add_event(str(quote_id), PRICING_FEE_QUOTE_ISSUED, payload)
                await self._publisher.publish(PRICING_FEE_QUOTE_ISSUED, payload)
                issued.append(
                    QuoteIssued(
                        quote_id=quote_id,
                        tier=tname,
                        rail=rname,
                        breakdown=qr.breakdown,
                        gross=qr.gross,
                        currency=currency,
                        gross_settled=qr.gross_settled,
                        settle_currency=settle_currency,
                        platform_fee=qr.platform_fee,
                        rail_fee=qr.rail_fee,
                        net=qr.net,
                        fx=fx,
                    )
                )
        await self._commit()
        log.info("quote_issued", merchant_id=str(merchant_id), count=len(issued))
        return issued
