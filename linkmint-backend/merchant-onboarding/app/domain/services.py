"""Wire the per-request service bundle from shared singletons + a session-bound repository.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
tests build the same bundle over in-memory fakes, so there is a single override point.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.config import Settings
from app.db.repositories import MerchantRepository
from app.domain.bank_accounts_service import BankAccountsService
from app.domain.contracts_service import ContractsService
from app.domain.documents_service import DocumentsService
from app.domain.merchants_service import MerchantsService
from app.events.publisher import Publisher
from app.security.bank_crypto import BankCipher
from app.storage.object_store import ObjectStore

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class ServiceDeps:
    repo: MerchantRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    bank_cipher: BankCipher
    object_store: ObjectStore


@dataclass(frozen=True)
class Services:
    merchants: MerchantsService
    bank_accounts: BankAccountsService
    documents: DocumentsService
    contracts: ContractsService


def build_services(d: ServiceDeps) -> Services:
    merchants = MerchantsService(d.repo, d.commit, d.publisher, d.settings)
    bank_accounts = BankAccountsService(d.repo, d.commit, d.bank_cipher, d.publisher)
    documents = DocumentsService(d.repo, d.commit, d.object_store, d.settings)
    contracts = ContractsService(d.repo, d.commit, d.publisher)
    return Services(
        merchants=merchants,
        bank_accounts=bank_accounts,
        documents=documents,
        contracts=contracts,
    )
