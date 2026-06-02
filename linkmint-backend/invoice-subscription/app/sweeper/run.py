"""Overdue sweeper — a lifespan task that flips OPEN invoices past ``due_at`` to OVERDUE.

Runs when ``INVOICE_OVERDUE_SWEEP_ENABLED=true`` (default). Each tick scans for due-past OPEN
invoices, marks them OVERDUE, and emits ``invoice.overdue`` (via the outbox). Reads already reflect
OVERDUE lazily; this task is what persists the transition and produces the event.
"""

from __future__ import annotations

import asyncio

from fastapi import FastAPI

from app.config import Settings
from app.db.repositories import InvoiceRepository
from app.domain.services import ServiceDeps, build_services
from app.logging import get_logger

log = get_logger("invoice.sweeper")


async def _sweep_once(app: FastAPI) -> int:
    settings: Settings = app.state.settings
    async with app.state.sessionmaker() as session:
        deps = ServiceDeps(
            repo=InvoiceRepository(session),
            commit=session.commit,
            settings=settings,
            publisher=app.state.publisher,
            paylink=app.state.paylink_client,
        )
        return await build_services(deps).invoices.sweep_overdue()


async def run(app: FastAPI) -> None:
    settings: Settings = app.state.settings
    log.info("sweeper_loop_started", interval=settings.overdue_sweep_interval_seconds)
    while True:
        try:
            await _sweep_once(app)
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 — a transient blip must not kill the loop
            log.warning("overdue_sweep_error", error=str(exc))
        await asyncio.sleep(settings.overdue_sweep_interval_seconds)
