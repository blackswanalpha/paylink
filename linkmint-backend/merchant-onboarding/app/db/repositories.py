"""Data access for the ``merchant`` schema.

One session-bound repository exposes every query the domain services need. Mutations follow the
reference pattern: ``insert_*`` adds + flushes; updates mutate a fetched row and rely on the
service's ``commit``. Tests substitute an in-memory fake with the same surface.
"""

from __future__ import annotations

import uuid
from typing import Any

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    BankAccountRow,
    ContractRow,
    DocumentRow,
    MerchantEventRow,
    MerchantRow,
)


class MerchantRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    # ── merchants ──
    async def insert_merchant(self, row: MerchantRow) -> MerchantRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_merchant(self, merchant_id: uuid.UUID) -> MerchantRow | None:
        return await self._session.get(MerchantRow, merchant_id)

    async def get_merchant_by_org(self, org_id: uuid.UUID) -> MerchantRow | None:
        stmt = select(MerchantRow).where(MerchantRow.org_id == org_id)
        return (await self._session.execute(stmt)).scalar_one_or_none()

    # ── bank accounts ──
    async def insert_bank_account(self, row: BankAccountRow) -> BankAccountRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def get_bank_account(self, bank_account_id: uuid.UUID) -> BankAccountRow | None:
        return await self._session.get(BankAccountRow, bank_account_id)

    async def list_bank_accounts(self, merchant_id: uuid.UUID) -> list[BankAccountRow]:
        stmt = select(BankAccountRow).where(BankAccountRow.merchant_id == merchant_id)
        return list((await self._session.execute(stmt)).scalars().all())

    async def count_verified_bank_accounts(self, merchant_id: uuid.UUID) -> int:
        stmt = (
            select(func.count())
            .select_from(BankAccountRow)
            .where(
                BankAccountRow.merchant_id == merchant_id,
                BankAccountRow.status == "VERIFIED",
            )
        )
        return int((await self._session.execute(stmt)).scalar_one())

    # ── documents ──
    async def insert_document(self, row: DocumentRow) -> DocumentRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def list_documents(self, merchant_id: uuid.UUID) -> list[DocumentRow]:
        stmt = select(DocumentRow).where(DocumentRow.merchant_id == merchant_id)
        return list((await self._session.execute(stmt)).scalars().all())

    # ── contracts ──
    async def insert_contract(self, row: ContractRow) -> ContractRow:
        self._session.add(row)
        await self._session.flush()
        return row

    async def list_contracts(self, merchant_id: uuid.UUID) -> list[ContractRow]:
        stmt = (
            select(ContractRow)
            .where(ContractRow.merchant_id == merchant_id)
            .order_by(ContractRow.accepted_at.desc())
        )
        return list((await self._session.execute(stmt)).scalars().all())

    async def count_contracts(self, merchant_id: uuid.UUID) -> int:
        stmt = (
            select(func.count())
            .select_from(ContractRow)
            .where(ContractRow.merchant_id == merchant_id)
        )
        return int((await self._session.execute(stmt)).scalar_one())

    # ── events (outbox) ──
    async def add_event(
        self,
        subject_type: str,
        subject_id: uuid.UUID | None,
        kind: str,
        payload: dict[str, Any],
    ) -> None:
        self._session.add(
            MerchantEventRow(
                subject_type=subject_type, subject_id=subject_id, kind=kind, payload=payload
            )
        )
        await self._session.flush()
