"""Off-chain domain types for PayLinks."""

from __future__ import annotations

from enum import StrEnum


class OffChainStatus(StrEnum):
    """The off-chain lifecycle (backendfeatures.md §2.2).

    CREATED  — row written, on-chain create tx not yet confirmed submitted
    PENDING  — on-chain create tx submitted; awaiting validator quorum
    VERIFIED — chain reached quorum (settled)
    FAILED   — chain marked failed / proof rejected
    CANCELLED— cancelled by creator/owner before settlement
    EXPIRED  — passed expiry while still PENDING
    """

    CREATED = "CREATED"
    PENDING = "PENDING"
    VERIFIED = "VERIFIED"
    FAILED = "FAILED"
    CANCELLED = "CANCELLED"
    EXPIRED = "EXPIRED"


# Terminal off-chain states (settlement decided on-chain — never re-derived locally; A.7).
TERMINAL: frozenset[OffChainStatus] = frozenset(
    {OffChainStatus.VERIFIED, OffChainStatus.FAILED, OffChainStatus.CANCELLED}
)


def is_terminal(status: OffChainStatus) -> bool:
    return status in TERMINAL


# On-chain Status string -> off-chain status (the create tx is in flight while chain == CREATED).
CHAIN_TO_OFFCHAIN: dict[str, OffChainStatus] = {
    "CREATED": OffChainStatus.PENDING,
    "VERIFIED": OffChainStatus.VERIFIED,
    "FAILED": OffChainStatus.FAILED,
    "CANCELLED": OffChainStatus.CANCELLED,
}
