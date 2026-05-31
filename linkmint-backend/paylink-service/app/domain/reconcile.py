"""Pure status reconciliation: chain truth → off-chain status (upholds invariant A.7)."""

from __future__ import annotations

from datetime import datetime

from app.chain.mapping import ChainPayLink
from app.domain.models import CHAIN_TO_OFFCHAIN, OffChainStatus, is_terminal


def reconcile_status(
    local: OffChainStatus, chain: ChainPayLink | None, *, now: datetime
) -> OffChainStatus:
    """Return the off-chain status given the local value and the on-chain truth.

    Settlement (VERIFIED/FAILED/CANCELLED) is taken *only* from the chain. A chain-side ``CREATED``
    maps to off-chain ``PENDING`` — or ``EXPIRED`` once wall-clock passes the PayLink's expiry.
    """
    if is_terminal(local):
        return local
    if chain is None:
        return local

    mapped = CHAIN_TO_OFFCHAIN.get(chain.status)
    if mapped in (OffChainStatus.VERIFIED, OffChainStatus.FAILED, OffChainStatus.CANCELLED):
        return mapped
    if chain.expiry and now.timestamp() > chain.expiry:
        return OffChainStatus.EXPIRED
    return OffChainStatus.PENDING
