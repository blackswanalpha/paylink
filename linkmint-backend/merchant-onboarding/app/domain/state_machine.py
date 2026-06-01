"""Merchant lifecycle state machine. Pure and exhaustively unit-tested.

``DRAFT → PENDING_VERIFICATION → {ACTIVE, REJECTED, SUSPENDED}``; ``ACTIVE → SUSPENDED``;
``SUSPENDED → {ACTIVE, REJECTED}``; ``REJECTED`` is terminal. ``onboard`` creates a merchant
*directly* at ``PENDING_VERIFICATION`` (so ``DRAFT`` exists for completeness / future self-serve
draft saves), and the API contract returns ``PENDING_VERIFICATION``.

A :class:`ReviewDecision` maps to a target status; :func:`assert_transition` enforces the allowed
edges, raising ``INVALID_TRANSITION`` (409) on a disallowed move. Activation *preconditions*
(verified bank + accepted contract) live in the service layer (env-gated), not here — this module
only knows the graph.
"""

from __future__ import annotations

from app.domain.models import MerchantStatus, ReviewDecision
from app.errors import AppError, ErrorCode

# Allowed status edges.
_EDGES: dict[MerchantStatus, set[MerchantStatus]] = {
    MerchantStatus.DRAFT: {MerchantStatus.PENDING_VERIFICATION},
    MerchantStatus.PENDING_VERIFICATION: {
        MerchantStatus.ACTIVE,
        MerchantStatus.REJECTED,
        MerchantStatus.SUSPENDED,
    },
    MerchantStatus.ACTIVE: {MerchantStatus.SUSPENDED},
    MerchantStatus.SUSPENDED: {MerchantStatus.ACTIVE, MerchantStatus.REJECTED},
    MerchantStatus.REJECTED: set(),  # terminal
}

# A review decision resolves to the target status it drives.
_DECISION_TARGET: dict[ReviewDecision, MerchantStatus] = {
    ReviewDecision.APPROVE: MerchantStatus.ACTIVE,
    ReviewDecision.REJECT: MerchantStatus.REJECTED,
    ReviewDecision.SUSPEND: MerchantStatus.SUSPENDED,
    ReviewDecision.REINSTATE: MerchantStatus.ACTIVE,
}


def can_transition(current: MerchantStatus, target: MerchantStatus) -> bool:
    return target in _EDGES.get(current, set())


def target_for(decision: ReviewDecision) -> MerchantStatus:
    return _DECISION_TARGET[decision]


def assert_transition(current: MerchantStatus, target: MerchantStatus) -> None:
    """Raise ``INVALID_TRANSITION`` (409) if ``current → target`` is not an allowed edge."""
    if not can_transition(current, target):
        raise AppError(
            ErrorCode.INVALID_TRANSITION,
            f"cannot transition merchant from {current.value} to {target.value}",
            details={"from": current.value, "to": target.value},
        )
