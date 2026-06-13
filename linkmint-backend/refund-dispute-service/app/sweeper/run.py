"""Lifespan sweeper — expires overdue disputes and (in dev) simulates refund completion.

Runs when ``REFUND_SWEEP_ENABLED=true``. Each tick (1) moves OPEN disputes past their evidence
window to EXPIRED and requests a clawback (treated as a loss), and (2) when
``REFUND_REVERSAL_SIMULATE``
is set, advances PROCESSING refunds older than the threshold to COMPLETED. The same ``_run_once`` is
unit-testable in isolation (mirrors fee-pricing-service's sweeper shape).
"""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import RefundRepository
from app.domain.services import ServiceDeps, build_services
from app.logging import get_logger

log = get_logger("refund.sweeper")


async def _run_once(app: FastAPI) -> tuple[int, int]:
    settings: Settings = app.state.settings
    now = datetime.now(UTC)
    async with app.state.sessionmaker() as session:
        deps = ServiceDeps(
            repo=RefundRepository(session),
            commit=session.commit,
            settings=settings,
            publisher=app.state.publisher,
            payments=app.state.payments_client,
            paylinks=app.state.paylinks_client,
            reversal=app.state.reversal_registry,
            clawback=app.state.clawback,
            ledger=app.state.ledger_poster,
        )
        services = build_services(deps)
        expired = await services.disputes.expire_due(now)
        completed = 0
        if settings.reversal_simulate:
            completed = await services.refunds.simulate_due_completions(now)
        return expired, completed


async def run(app: FastAPI) -> None:
    settings: Settings = app.state.settings
    log.info("sweeper_started", interval=settings.sweep_interval_seconds)
    while True:
        try:
            expired, completed = await _run_once(app)
            if expired or completed:
                log.info("sweep_tick", expired=expired, completed=completed)
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 — a transient blip must not kill the loop
            log.warning("sweep_error", error=str(exc))
        await asyncio.sleep(settings.sweep_interval_seconds)
