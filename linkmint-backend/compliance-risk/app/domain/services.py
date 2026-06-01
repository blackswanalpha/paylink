"""Wire the per-request service bundle from shared singletons + a session-bound repository.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
tests build the same bundle over in-memory fakes, so there is a single override point.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.config import Settings
from app.db.repositories import ComplianceRepository
from app.domain.kyc_service import KycService
from app.domain.risk_service import RiskService
from app.events.publisher import Publisher
from app.security.provider_crypto import ProviderCipher

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class ServiceDeps:
    repo: ComplianceRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    cipher: ProviderCipher


@dataclass(frozen=True)
class Services:
    kyc: KycService
    risk: RiskService


def build_services(d: ServiceDeps) -> Services:
    kyc = KycService(d.repo, d.commit, d.publisher, d.cipher, d.settings)
    risk = RiskService(d.repo, d.commit, d.publisher, d.settings)
    return Services(kyc=kyc, risk=risk)
