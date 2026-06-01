"""Shared test doubles + helpers (imported by conftest and integration tests)."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from app.compliance.client import ComplianceUnavailable, RiskDecision
from app.config import Settings
from app.db.models import PayLinkRow
from app.db.repository import decode_cursor, encode_cursor
from app.errors import AppError, ErrorCode

# Golden P-256 key whose address/signable/hash were captured from the Go chain (see test_signer).
GOLDEN_KEY = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"


async def noop_commit() -> None:
    return None


class FakeChainClient:
    """In-memory stand-in for the lVM JSON-RPC client."""

    def __init__(self) -> None:
        self.sent: list[dict[str, Any]] = []
        self.nonce = 0
        self.height = 1
        self.fail_send = False
        self.paylinks: dict[str, dict[str, Any]] = {}

    async def chain_height(self) -> int:
        return self.height

    async def get_nonce(self, address: str) -> int:
        return self.nonce

    async def send_transaction(self, tx: dict[str, Any]) -> str:
        if self.fail_send:
            raise AppError(ErrorCode.CHAIN_UNAVAILABLE, "chain down (test)")
        self.sent.append(tx)
        self.nonce += 1
        return str(tx.get("hash", "0xdeadbeef"))

    async def get_paylink(self, pl_id: str) -> dict[str, Any] | None:
        return self.paylinks.get(pl_id)

    async def get_paylinks_by_creator(self, *a: Any, **k: Any) -> list[dict[str, Any]]:
        return []

    async def get_paylinks_by_receiver(self, *a: Any, **k: Any) -> list[dict[str, Any]]:
        return []

    async def get_paylinks_by_status(self, *a: Any, **k: Any) -> list[dict[str, Any]]:
        return []

    async def get_receipt(self, tx_hash: str) -> dict[str, Any] | None:
        return None


class FakeComplianceClient:
    """In-memory stand-in for the compliance-risk ``/v1/risk/evaluate`` client."""

    def __init__(
        self,
        *,
        decision: str = "allow",
        score: float = 0.0,
        reasons: list[dict[str, Any]] | None = None,
        raise_unavailable: bool = False,
    ) -> None:
        self.decision = decision
        self.score = score
        self.reasons = reasons or []
        self.raise_unavailable = raise_unavailable
        self.calls: list[dict[str, Any]] = []

    async def evaluate(self, **kwargs: Any) -> RiskDecision:
        self.calls.append(kwargs)
        if self.raise_unavailable:
            raise ComplianceUnavailable("compliance down (test)")
        return RiskDecision(decision=self.decision, score=self.score, reasons=self.reasons)


class FakeRepository:
    """In-memory PayLink repository mirroring the real cursor semantics."""

    def __init__(self) -> None:
        self.rows: dict[str, PayLinkRow] = {}
        self.events: list[tuple[str, str, dict[str, Any]]] = []

    async def insert(self, row: PayLinkRow) -> PayLinkRow:
        now = datetime.now(UTC)
        if row.created_at is None:
            row.created_at = now
        if row.updated_at is None:
            row.updated_at = now
        if row.vote_count is None:
            row.vote_count = 0
        self.rows[row.pl_id] = row
        return row

    async def get(self, pl_id: str) -> PayLinkRow | None:
        return self.rows.get(pl_id)

    async def add_event(self, pl_id: str, kind: str, payload: dict[str, Any]) -> None:
        self.events.append((pl_id, kind, payload))

    async def list_paylinks(
        self,
        *,
        creator: str | None = None,
        receiver: str | None = None,
        status: str | None = None,
        limit: int = 20,
        cursor: str | None = None,
    ) -> tuple[list[PayLinkRow], str | None]:
        items = [
            r
            for r in self.rows.values()
            if (creator is None or r.creator_addr == creator)
            and (receiver is None or r.receiver_addr == receiver)
            and (status is None or r.status == status)
        ]
        items.sort(key=lambda r: (r.created_at, r.pl_id), reverse=True)
        if cursor:
            c_ts, c_id = decode_cursor(cursor)
            items = [r for r in items if (r.created_at, r.pl_id) < (c_ts, c_id)]
        next_cursor: str | None = None
        if len(items) > limit:
            items = items[:limit]
            next_cursor = encode_cursor(items[-1].created_at, items[-1].pl_id)
        return items, next_cursor


def make_settings(**overrides: Any) -> Settings:
    base: dict[str, Any] = {
        "database_url": "postgresql+psycopg://test:test@localhost:5432/test",
        "redis_url": "redis://localhost:6379/0",
        "chain_rpc_url": "http://localhost:8545/",
        "chain_submit_enabled": True,
        "signer_mode": "service_key",
        "chain_signer_key": None,
        "event_publisher_mode": "noop",
    }
    base.update(overrides)
    return Settings(**base)
