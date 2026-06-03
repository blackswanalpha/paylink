"""Monthly platform-fee invoice sweeper — a lifespan task that generates the current period's
platform-fee invoices without an external cron.

Runs when ``PRICING_INVOICE_SWEEP_ENABLED=true``. Each tick generates invoices for the current
``YYYY-MM`` period; generation is idempotent per merchant+period, so re-runs are no-ops. The same
``generate_for_period`` is what the internal ``/v1/internal/invoices/platform-fee/run`` endpoint
calls — one source of lifecycle.
"""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import PricingRepository
from app.domain.services import ServiceDeps, build_services
from app.logging import get_logger

log = get_logger("pricing.sweeper")


async def _run_once(app: FastAPI) -> int:
    settings: Settings = app.state.settings
    period = datetime.now(UTC).strftime("%Y-%m")
    async with app.state.sessionmaker() as session:
        deps = ServiceDeps(
            repo=PricingRepository(session),
            commit=session.commit,
            settings=settings,
            publisher=app.state.publisher,
            fx_provider=app.state.fx_provider,
            redis=app.state.redis,
            ledger=app.state.ledger_poster,
        )
        result = await build_services(deps).invoicing.generate_for_period(period)
        return len(result.generated)


async def run(app: FastAPI) -> None:
    settings: Settings = app.state.settings
    log.info("invoice_sweeper_started", interval=settings.invoice_sweep_interval_seconds)
    while True:
        try:
            await _run_once(app)
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 — a transient blip must not kill the loop
            log.warning("invoice_sweep_error", error=str(exc))
        await asyncio.sleep(settings.invoice_sweep_interval_seconds)
