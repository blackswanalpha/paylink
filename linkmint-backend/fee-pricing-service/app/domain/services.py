"""Per-request service bundle.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
the bus consumer and the invoice sweeper build the same bundle over a fresh session, so the rules
live in one place. Tests build the bundle over in-memory fakes (same surface).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.config import Settings
from app.db.repositories import PricingRepository
from app.domain.fx_service import FxService, RedisLike
from app.domain.invoicing_service import InvoicingService
from app.domain.pricing_service import PricingService
from app.events.publisher import Publisher
from app.fx.provider import FxProvider
from app.ledger.poster import LedgerPoster

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class ServiceDeps:
    repo: PricingRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    fx_provider: FxProvider
    redis: RedisLike
    ledger: LedgerPoster


@dataclass(frozen=True)
class Services:
    pricing: PricingService
    fx: FxService
    invoicing: InvoicingService


def build_services(d: ServiceDeps) -> Services:
    fx = FxService(d.repo, d.commit, d.publisher, d.fx_provider, d.redis, d.settings)
    pricing = PricingService(d.repo, d.commit, d.publisher, fx, d.settings)
    invoicing = InvoicingService(d.repo, d.commit, d.publisher, d.ledger, d.settings)
    return Services(pricing=pricing, fx=fx, invoicing=invoicing)
