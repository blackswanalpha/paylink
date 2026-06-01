"""Bank-account linking + verification.

INVARIANT (rules.md Part A — non-custodial): the request's ``account_details`` is encrypted to the
``account_ref`` column IMMEDIATELY via the KMS-stand-in cipher, and the plaintext is NEVER
persisted, logged, returned in any response, or placed in an event payload. Callers get ids/status.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime

from app.db.models import BankAccountRow
from app.db.repositories import MerchantRepository
from app.domain import rbac
from app.domain.models import BankAccountStatus, Rail
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.bank_crypto import BankCipher
from app.security.jwt import AccessClaims

_Commit = Callable[[], Awaitable[None]]


class BankAccountsService:
    def __init__(
        self,
        repo: MerchantRepository,
        commit: _Commit,
        cipher: BankCipher,
        publisher: Publisher,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._cipher = cipher
        self._publisher = publisher

    async def _merchant_for_member(
        self, principal: AccessClaims, merchant_id: uuid.UUID
    ) -> uuid.UUID:
        """Return the merchant's org_id if the caller may see it, else MERCHANT_NOT_FOUND."""
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None or rbac.org_role(principal, str(merchant.org_id)) is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")
        return merchant.org_id

    async def add_bank_account(
        self,
        *,
        principal: AccessClaims,
        merchant_id: uuid.UUID,
        rail: str,
        account_details: str,
        currency: str,
        country: str,
    ) -> BankAccountRow:
        await self._merchant_for_member(principal, merchant_id)
        if rail not in set(Rail):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid rail '{rail}'")
        if not _valid_account_details(account_details):
            raise AppError(ErrorCode.INVALID_ACCOUNT, "invalid account details")

        # Encrypt the plaintext account details to the at-rest ref IMMEDIATELY — the plaintext is
        # never written to the DB, a log, a response, or an event payload.
        account_ref = self._cipher.encrypt(account_details.strip())
        account = BankAccountRow(
            bank_account_id=uuid.uuid4(),
            merchant_id=merchant_id,
            rail=rail,
            account_ref=account_ref,
            currency=currency.strip().upper(),
            status=BankAccountStatus.PENDING_VERIFY.value,
        )
        await self._repo.insert_bank_account(account)
        payload = {
            "merchant_id": str(merchant_id),
            "bank_account_id": str(account.bank_account_id),
            "rail": rail,
            "currency": account.currency,
            "status": account.status,
        }  # NB: no account_details / account_ref in the payload
        await self._repo.add_event(
            "bank_account", account.bank_account_id, events.BANK_ACCOUNT_ADDED, payload
        )
        await self._commit()
        await self._publisher.publish(events.BANK_ACCOUNT_ADDED, payload)
        return account

    async def verify_bank_account(
        self,
        *,
        principal: AccessClaims,
        merchant_id: uuid.UUID,
        bank_account_id: uuid.UUID,
    ) -> BankAccountRow:
        await self._merchant_for_member(principal, merchant_id)
        account = await self._repo.get_bank_account(bank_account_id)
        if account is None or account.merchant_id != merchant_id:
            raise AppError(ErrorCode.BANK_ACCOUNT_NOT_FOUND, "bank account not found")
        if account.status != BankAccountStatus.PENDING_VERIFY.value:
            raise AppError(
                ErrorCode.INVALID_TRANSITION,
                f"bank account is not pending verification (status={account.status})",
                details={"status": account.status},
            )
        # Phase-1 manual verification stub (KE/MPesa B2B paybill name-match / micro-deposit lands
        # with the bank-verification adapters in a later work item). Here, verify marks VERIFIED.
        account.status = BankAccountStatus.VERIFIED.value
        account.verified_at = datetime.now(UTC)
        payload = {
            "merchant_id": str(merchant_id),
            "bank_account_id": str(account.bank_account_id),
            "status": account.status,
        }
        await self._repo.add_event(
            "bank_account", account.bank_account_id, events.BANK_ACCOUNT_VERIFIED, payload
        )
        await self._commit()
        await self._publisher.publish(events.BANK_ACCOUNT_VERIFIED, payload)
        return account

    async def list_bank_accounts(
        self, *, principal: AccessClaims, merchant_id: uuid.UUID
    ) -> list[BankAccountRow]:
        await self._merchant_for_member(principal, merchant_id)
        return await self._repo.list_bank_accounts(merchant_id)


def _valid_account_details(account_details: str) -> bool:
    """Minimal structural validation (rail-agnostic): non-empty, reasonable length.

    The real per-rail validation (IBAN check digits, MPesa shortcode/MSISDN shape) lands with the
    bank-verification adapters; here we reject obviously empty/oversized inputs.
    """
    s = account_details.strip()
    return 0 < len(s) <= 256
