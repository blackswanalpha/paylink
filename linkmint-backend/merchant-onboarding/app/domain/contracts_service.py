"""Contract acceptance + listing.

Records who accepted (``accepted_by`` — an opaque ref to identity.users, NO cross-schema FK), the
client IP and user-agent. Emits ``merchant.contract.accepted`` via the outbox.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable

from app.db.models import ContractRow
from app.db.repositories import MerchantRepository
from app.domain import rbac
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.jwt import AccessClaims

_Commit = Callable[[], Awaitable[None]]


class ContractsService:
    def __init__(self, repo: MerchantRepository, commit: _Commit, publisher: Publisher) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher

    async def _merchant_for_member(self, principal: AccessClaims, merchant_id: uuid.UUID) -> None:
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None or rbac.org_role(principal, str(merchant.org_id)) is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")

    async def accept_contract(
        self,
        *,
        principal: AccessClaims,
        merchant_id: uuid.UUID,
        version: str,
        accepted: bool,
        ip: str | None,
        user_agent: str | None,
    ) -> ContractRow:
        await self._merchant_for_member(principal, merchant_id)
        if accepted is not True:
            raise AppError(ErrorCode.INVALID_PAYLOAD, "contract must be explicitly accepted")
        contract = ContractRow(
            merchant_id=merchant_id,
            version=version,
            accepted_by=uuid.UUID(principal.user_id),
            ip=ip,
            user_agent=user_agent,
        )
        await self._repo.insert_contract(contract)
        payload = {
            "merchant_id": str(merchant_id),
            "version": version,
            "accepted_by": principal.user_id,
        }
        await self._repo.add_event("merchant", merchant_id, events.CONTRACT_ACCEPTED, payload)
        await self._commit()
        await self._publisher.publish(events.CONTRACT_ACCEPTED, payload)
        return contract

    async def list_contracts(
        self, *, principal: AccessClaims, merchant_id: uuid.UUID
    ) -> list[ContractRow]:
        await self._merchant_for_member(principal, merchant_id)
        return await self._repo.list_contracts(merchant_id)
