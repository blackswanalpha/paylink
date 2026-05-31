"""PayLink use-cases: create / get / list / cancel, plus on-chain status read-through."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from decimal import Decimal
from typing import Any, Protocol

from app.chain import tx_builder
from app.chain.client import ChainClient
from app.chain.mapping import ChainPayLink
from app.chain.nonce import NonceManager
from app.chain.signer import Signer
from app.config import Settings
from app.db.models import PayLinkRow
from app.domain import reconcile
from app.domain.models import OffChainStatus, is_terminal
from app.errors import AppError, ErrorCode
from app.events import publisher as ev
from app.events.publisher import Publisher
from app.logging import get_logger

log = get_logger("paylink.service")


@dataclass
class CreateCommand:
    receiver: str
    amount: int
    currency: str
    expiry: datetime
    usage: str
    metadata: dict[str, Any] | None
    rules: Any | None
    idem_key: str | None
    caller_addr: str


class Repository(Protocol):
    """The repository surface the service needs (real impl: :class:`PayLinkRepository`)."""

    async def insert(self, row: PayLinkRow) -> PayLinkRow: ...
    async def get(self, pl_id: str) -> PayLinkRow | None: ...
    async def add_event(self, pl_id: str, kind: str, payload: dict[str, Any]) -> None: ...
    async def list_paylinks(
        self,
        *,
        creator: str | None = ...,
        receiver: str | None = ...,
        status: str | None = ...,
        limit: int = ...,
        cursor: str | None = ...,
    ) -> tuple[list[PayLinkRow], str | None]: ...


class PayLinkService:
    def __init__(
        self,
        repo: Repository,
        commit: Callable[[], Awaitable[None]],
        chain: ChainClient,
        signer: Signer,
        nonces: NonceManager,
        publisher: Publisher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._chain = chain
        self._signer = signer
        self._nonces = nonces
        self._publisher = publisher
        self._settings = settings

    # ── create ──
    async def create(self, cmd: CreateCommand) -> PayLinkRow:
        creator = cmd.caller_addr
        expiry_unix = int(cmd.expiry.timestamp())
        pl_id = tx_builder.new_pl_id(
            creator, cmd.receiver, cmd.amount, expiry_unix, seed=cmd.idem_key
        )
        md_hash = tx_builder.metadata_hash(cmd.metadata)

        # SEAM (work12): compliance/KYC gate — when amount > threshold, a synchronous call to
        # compliance-risk would gate creation (402 KYC_REQUIRED). Deferred; threshold is configured.

        requested_payload = {
            "pl_id": pl_id,
            "creator": creator,
            "receiver": cmd.receiver,
            "amount": cmd.amount,
            "currency": cmd.currency,
            "expiry": expiry_unix,
            "usage": cmd.usage,
        }
        row = PayLinkRow(
            pl_id=pl_id,
            creator_addr=creator,
            receiver_addr=cmd.receiver,
            owner_addr=creator,  # owner initially equals creator
            amount=Decimal(cmd.amount),
            currency=cmd.currency,
            status=OffChainStatus.CREATED.value,
            expiry=cmd.expiry,
            usage=cmd.usage,
            meta=cmd.metadata,
            rules=cmd.rules,
            chain_tx_hash=None,
            vote_count=0,
        )
        await self._repo.insert(row)
        await self._repo.add_event(pl_id, ev.PAYLINK_REQUESTED, requested_payload)
        await self._commit()
        await self._publisher.publish(ev.PAYLINK_REQUESTED, requested_payload)

        if self._settings.chain_submit_enabled:
            chain_tx_hash = await self._submit_create(
                pl_id, cmd.receiver, cmd.amount, expiry_unix, md_hash, cmd.rules
            )
            row.status = OffChainStatus.PENDING.value
            row.chain_tx_hash = chain_tx_hash
            created_payload = {"pl_id": pl_id, "chain_tx_hash": chain_tx_hash}
            await self._repo.add_event(pl_id, ev.PAYLINK_CREATED, created_payload)
            await self._commit()
            await self._publisher.publish(ev.PAYLINK_CREATED, created_payload)
        return row

    async def _submit_create(
        self,
        pl_id: str,
        receiver: str,
        amount: int,
        expiry_unix: int,
        md_hash: str,
        rules: Any | None,
    ) -> str:
        async with self._nonces.reserve(self._signer.address) as nonce:
            core = tx_builder.build_create(
                pl_id=pl_id,
                from_addr=self._signer.address,
                nonce=nonce,
                receiver=receiver,
                amount=amount,
                expiry=expiry_unix,
                md_hash=md_hash,
                rules=rules,
            )
            tx = tx_builder.sign_tx(core, self._signer)
            return await self._chain.send_transaction(tx)

    # ── read ──
    async def get(self, pl_id: str) -> PayLinkRow:
        row = await self._repo.get(pl_id)
        if row is None:
            raise AppError(ErrorCode.PAYLINK_NOT_FOUND, f"paylink not found: {pl_id}")
        await self._reconcile(row)
        return row

    async def list(
        self,
        *,
        creator: str | None,
        receiver: str | None,
        status: str | None,
        limit: int,
        cursor: str | None,
    ) -> tuple[list[PayLinkRow], str | None]:
        if status is not None and status not in {s.value for s in OffChainStatus}:
            raise AppError(ErrorCode.INVALID_QUERY, f"invalid status filter: {status}")
        return await self._repo.list_paylinks(
            creator=creator, receiver=receiver, status=status, limit=limit, cursor=cursor
        )

    async def _reconcile(self, row: PayLinkRow) -> None:
        """Refresh a single PayLink's status from the chain (best-effort; never invents status)."""
        local = OffChainStatus(row.status)
        if is_terminal(local) or not row.chain_tx_hash:
            return
        try:
            chain_resp = await self._chain.get_paylink(row.pl_id)
        except AppError as exc:
            if exc.code is ErrorCode.CHAIN_UNAVAILABLE:
                log.warning("reconcile_skip_chain_unavailable", pl_id=row.pl_id)
                return
            raise
        chain_pl = ChainPayLink.from_rpc(chain_resp) if chain_resp else None
        now = datetime.now(UTC)
        new = reconcile.reconcile_status(local, chain_pl, now=now)
        if new == local:
            return
        row.status = new.value
        if chain_pl is not None:
            row.vote_count = chain_pl.vote_count
        if new is OffChainStatus.VERIFIED:
            row.verified_at = now
        if new is OffChainStatus.EXPIRED:
            await self._repo.add_event(row.pl_id, ev.PAYLINK_EXPIRED, {"pl_id": row.pl_id})
        await self._commit()
        if new is OffChainStatus.EXPIRED:
            await self._publisher.publish(ev.PAYLINK_EXPIRED, {"pl_id": row.pl_id})

    # ── cancel ──
    async def cancel(self, pl_id: str, caller_addr: str) -> PayLinkRow:
        row = await self._repo.get(pl_id)
        if row is None:
            raise AppError(ErrorCode.PAYLINK_NOT_FOUND, f"paylink not found: {pl_id}")

        # Mirror the on-chain guard: only the creator or current owner may cancel.
        if caller_addr not in (row.creator_addr, row.owner_addr):
            raise AppError(ErrorCode.UNAUTHORIZED, "only the creator or owner may cancel")

        local = OffChainStatus(row.status)
        if local is OffChainStatus.CANCELLED:
            return row  # idempotent
        if local in (OffChainStatus.VERIFIED, OffChainStatus.FAILED):
            raise AppError(
                ErrorCode.PAYLINK_ALREADY_SETTLED, f"paylink already {local.value.lower()}"
            )
        if local is OffChainStatus.EXPIRED:
            raise AppError(ErrorCode.PAYLINK_EXPIRED, "paylink has expired")

        if self._settings.chain_submit_enabled and row.chain_tx_hash:
            await self._submit_cancel(pl_id)

        row.status = OffChainStatus.CANCELLED.value
        cancelled_payload = {"pl_id": pl_id, "by": caller_addr}
        await self._repo.add_event(pl_id, ev.PAYLINK_CANCELLED, cancelled_payload)
        await self._commit()
        await self._publisher.publish(ev.PAYLINK_CANCELLED, cancelled_payload)
        return row

    async def _submit_cancel(self, pl_id: str) -> str:
        async with self._nonces.reserve(self._signer.address) as nonce:
            core = tx_builder.build_cancel(pl_id=pl_id, from_addr=self._signer.address, nonce=nonce)
            tx = tx_builder.sign_tx(core, self._signer)
            return await self._chain.send_transaction(tx)
