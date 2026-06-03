"""Double-entry ledger seam (rules.md A.6) — OFF by default.

A platform-fee invoice is a monetary flow, so A.6 says it should eventually have matching
debit/credit entries. But work16 explicitly defers per-service ledger posting (the settlement hub is
the intended writer; only payment-orchestrator gates on ``ledger-migrate``). So we ship a port + a
no-op default and gate the call on ``PRICING_LEDGER_POSTING_ENABLED`` (false): the seam honors A.6
intent and makes the future switch a one-line config + a real ledger-python adapter, with zero
coupling to ``ledger-migrate`` now and no risk of double-posting.
"""

from __future__ import annotations

import uuid
from decimal import Decimal
from typing import Protocol

from app.logging import get_logger

log = get_logger("pricing.ledger")


class LedgerPoster(Protocol):
    async def post_platform_fee(
        self,
        *,
        invoice_id: uuid.UUID,
        merchant_id: uuid.UUID,
        period: str,
        currency: str,
        total_fee: Decimal,
    ) -> None: ...


class NoopLedgerPoster:
    """The default poster — records nothing, logs the intent. Swap for a ledger-python adapter when
    work16 per-service posting is enabled."""

    async def post_platform_fee(
        self,
        *,
        invoice_id: uuid.UUID,
        merchant_id: uuid.UUID,
        period: str,
        currency: str,
        total_fee: Decimal,
    ) -> None:
        log.info(
            "ledger_post_skipped",
            invoice_id=str(invoice_id),
            merchant_id=str(merchant_id),
            period=period,
            currency=currency,
            total_fee=str(total_fee),
        )
