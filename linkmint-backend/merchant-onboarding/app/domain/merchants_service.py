"""Merchant lifecycle: onboard, read, fee-tier, and the guarded state-machine ``decide``.

Every mutation follows the reference outbox pattern (identity-service ``OrgsService``): mutate +
``repo.add_event(...)`` in one transaction → ``commit()`` → ``publisher.publish(...)`` post-commit.
RBAC is enforced from the token claims (see :mod:`app.domain.rbac`).
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime

from app.config import Settings
from app.db.models import BankAccountRow, MerchantRow
from app.db.repositories import MerchantRepository
from app.domain import rbac, state_machine
from app.domain.models import FeeTier, MerchantStatus, MerchantType, ReviewDecision
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.jwt import AccessClaims

_Commit = Callable[[], Awaitable[None]]


class MerchantsService:
    def __init__(
        self,
        repo: MerchantRepository,
        commit: _Commit,
        publisher: Publisher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._settings = settings

    async def onboard(
        self,
        *,
        principal: AccessClaims,
        org_id: uuid.UUID,
        business_name: str,
        registration_no: str | None,
        country: str,
        merchant_type: str,
    ) -> MerchantRow:
        rbac.require_org_member(principal, str(org_id))
        if merchant_type not in set(MerchantType):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid merchant type '{merchant_type}'")
        country_code = country.strip().upper()
        if country_code not in self._settings.allowed_country_set():
            raise AppError(
                ErrorCode.UNSUPPORTED_COUNTRY,
                f"country '{country_code}' is not supported (Phase 1)",
                details={"country": country_code},
            )
        if await self._repo.get_merchant_by_org(org_id) is not None:
            raise AppError(ErrorCode.ALREADY_ONBOARDED, "this organization is already onboarded")

        merchant = MerchantRow(
            merchant_id=uuid.uuid4(),
            org_id=org_id,
            business_name=business_name,
            registration_no=registration_no,
            tax_id=None,
            country=country_code,
            type=merchant_type,
            # Onboard creates directly at PENDING_VERIFICATION (the API contract returns it).
            status=MerchantStatus.PENDING_VERIFICATION.value,
            fee_tier=FeeTier.STANDARD.value,
        )
        await self._repo.insert_merchant(merchant)
        payload = {
            "merchant_id": str(merchant.merchant_id),
            "org_id": str(org_id),
            "country": country_code,
            "type": merchant_type,
            "status": merchant.status,
        }
        await self._repo.add_event(
            "merchant", merchant.merchant_id, events.MERCHANT_ONBOARDED, payload
        )
        await self._commit()
        await self._publisher.publish(events.MERCHANT_ONBOARDED, payload)
        return merchant

    async def get_for_member(
        self, *, principal: AccessClaims, merchant_id: uuid.UUID
    ) -> MerchantRow:
        """Fetch a merchant the caller may see, else ``MERCHANT_NOT_FOUND`` (no existence leak)."""
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None or rbac.org_role(principal, str(merchant.org_id)) is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")
        return merchant

    async def get_admin(self, merchant_id: uuid.UUID) -> MerchantRow:
        """Fetch any merchant for the internal/admin surface (no org-RBAC; transport authorizes)."""
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")
        return merchant

    async def list_bank_accounts_admin(self, merchant_id: uuid.UUID) -> list[BankAccountRow]:
        """Bank accounts for the internal/admin surface (id/status only — never the ref)."""
        return await self._repo.list_bank_accounts(merchant_id)

    async def search(self, q: str, limit: int = 20) -> list[MerchantRow]:
        """Admin search by business_name/registration_no substring or exact merchant_id/org_id."""
        return await self._repo.search_merchants(q, limit)

    async def set_fee_tier(
        self, *, principal: AccessClaims, merchant_id: uuid.UUID, tier: str
    ) -> MerchantRow:
        merchant = await self.get_for_member(principal=principal, merchant_id=merchant_id)
        rbac.require_admin(principal, str(merchant.org_id))
        if tier not in set(FeeTier):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid fee tier '{tier}'")
        merchant.fee_tier = tier
        payload = {"merchant_id": str(merchant.merchant_id), "tier": tier}
        await self._repo.add_event(
            "merchant", merchant.merchant_id, events.FEE_TIER_CHANGED, payload
        )
        await self._commit()
        await self._publisher.publish(events.FEE_TIER_CHANGED, payload)
        return merchant

    async def decide(
        self,
        *,
        merchant_id: uuid.UUID,
        decision: ReviewDecision,
        reason: str | None = None,
    ) -> MerchantRow:
        """Drive the state machine from a manual-review or consumer decision.

        This is the SINGLE guarded entry point shared by the ``/internal`` endpoint and the
        ``compliance.kyb.*`` / ``admin.override.*`` consumer. It is intentionally NOT RBAC-gated by
        a principal — it sits behind the internal/consumer surface (like the health probes), so the
        gateway/transport authorizes the caller. Activation to ACTIVE enforces the (env-gated)
        verified-bank + accepted-contract preconditions.
        """
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")
        current = MerchantStatus(merchant.status)
        target = state_machine.target_for(decision)
        state_machine.assert_transition(current, target)

        if target == MerchantStatus.ACTIVE:
            await self._assert_activation_preconditions(merchant_id)

        merchant.status = target.value
        now = datetime.now(UTC)
        if target == MerchantStatus.ACTIVE:
            merchant.onboarded_at = merchant.onboarded_at or now
            merchant.suspended_at = None
            merchant.suspended_reason = None
        elif target == MerchantStatus.SUSPENDED:
            merchant.suspended_at = now
            merchant.suspended_reason = reason
        elif target == MerchantStatus.REJECTED:
            merchant.suspended_reason = reason

        name = {
            MerchantStatus.ACTIVE: events.MERCHANT_VERIFIED,
            MerchantStatus.REJECTED: events.MERCHANT_REJECTED,
            MerchantStatus.SUSPENDED: events.MERCHANT_SUSPENDED,
        }[target]
        payload: dict[str, object] = {
            "merchant_id": str(merchant.merchant_id),
            "status": target.value,
            "decision": decision.value,
        }
        if reason is not None:
            payload["reason"] = reason
        await self._repo.add_event("merchant", merchant.merchant_id, name, payload)
        await self._commit()
        await self._publisher.publish(name, payload)
        return merchant

    async def _assert_activation_preconditions(self, merchant_id: uuid.UUID) -> None:
        """≥1 VERIFIED bank account + ≥1 accepted contract (each env-toggleable)."""
        if (
            self._settings.require_verified_bank_for_active
            and await self._repo.count_verified_bank_accounts(merchant_id) < 1
        ):
            raise AppError(
                ErrorCode.INVALID_TRANSITION,
                "cannot activate: no verified bank account",
                details={"missing": "verified_bank_account"},
            )
        if (
            self._settings.require_contract_for_active
            and await self._repo.count_contracts(merchant_id) < 1
        ):
            raise AppError(
                ErrorCode.INVALID_TRANSITION,
                "cannot activate: no accepted contract",
                details={"missing": "accepted_contract"},
            )
