"""Per-request service bundle.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
the bus consumer and the sweeper build the same bundle over a fresh session, so the rules live in
one
place. Tests build the bundle over in-memory fakes (same surface).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.clawback.coordinator import ClawbackCoordinator
from app.config import Settings
from app.db.repositories import RefundRepository
from app.domain.dispute_service import DisputeService
from app.domain.refund_service import RefundService
from app.events.publisher import Publisher
from app.ledger.poster import LedgerPoster
from app.paylinks.client import PaylinksClient
from app.payments.client import PaymentsClient
from app.reversal.port import RailReversalRegistry

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class ServiceDeps:
    repo: RefundRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    payments: PaymentsClient
    paylinks: PaylinksClient
    reversal: RailReversalRegistry
    clawback: ClawbackCoordinator
    ledger: LedgerPoster


@dataclass(frozen=True)
class Services:
    refunds: RefundService
    disputes: DisputeService


def build_services(d: ServiceDeps) -> Services:
    refunds = RefundService(
        d.repo,
        d.commit,
        d.publisher,
        d.settings,
        d.payments,
        d.paylinks,
        d.reversal,
        d.clawback,
        d.ledger,
    )
    disputes = DisputeService(d.repo, d.commit, d.publisher, d.settings, d.clawback)
    return Services(refunds=refunds, disputes=disputes)
