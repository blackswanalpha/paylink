"""Pure domain model: the refund + dispute state machines and small value helpers.

No I/O here — these functions are total and unit-tested directly. The service layer checks a
transition before mutating a row, so an illegal move surfaces as ``INVALID_STATE_TRANSITION`` (409)
rather than a silent corruption.
"""

from __future__ import annotations

from enum import StrEnum


class RefundState(StrEnum):
    REQUESTED = "REQUESTED"
    APPROVED = "APPROVED"
    REJECTED = "REJECTED"
    PROCESSING = "PROCESSING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"


class DisputeState(StrEnum):
    OPEN = "OPEN"
    SUBMITTED = "SUBMITTED"
    WON = "WON"
    LOST = "LOST"
    EXPIRED = "EXPIRED"


# Allowed transitions. ``approve`` short-circuits REQUESTED→PROCESSING (the reversal instruction is
# emitted in the same transition); APPROVED is retained as a legal intermediate for forward-compat.
_REFUND_TRANSITIONS: dict[RefundState, frozenset[RefundState]] = {
    RefundState.REQUESTED: frozenset(
        {RefundState.APPROVED, RefundState.REJECTED, RefundState.PROCESSING}
    ),
    RefundState.APPROVED: frozenset({RefundState.PROCESSING, RefundState.FAILED}),
    RefundState.PROCESSING: frozenset({RefundState.COMPLETED, RefundState.FAILED}),
    RefundState.REJECTED: frozenset(),
    RefundState.COMPLETED: frozenset(),
    RefundState.FAILED: frozenset(),
}

_DISPUTE_TRANSITIONS: dict[DisputeState, frozenset[DisputeState]] = {
    DisputeState.OPEN: frozenset(
        {DisputeState.SUBMITTED, DisputeState.WON, DisputeState.LOST, DisputeState.EXPIRED}
    ),
    DisputeState.SUBMITTED: frozenset({DisputeState.WON, DisputeState.LOST}),
    DisputeState.WON: frozenset(),
    DisputeState.LOST: frozenset(),
    DisputeState.EXPIRED: frozenset(),
}

# Dispute outcomes that trigger a clawback from the merchant's next payout (work23).
DISPUTE_LOSS_STATES = frozenset({DisputeState.LOST, DisputeState.EXPIRED})


def refund_can_transition(current: RefundState, target: RefundState) -> bool:
    return target in _REFUND_TRANSITIONS.get(current, frozenset())


def dispute_can_transition(current: DisputeState, target: DisputeState) -> bool:
    return target in _DISPUTE_TRANSITIONS.get(current, frozenset())


def is_partial_refund(amount_minor: int, original_minor: int | None) -> bool:
    """A refund is partial when its amount is below the resolvable original; when the original is
    unknown we cannot prove fullness, so it is treated as partial (conservative)."""
    if original_minor is None:
        return True
    return amount_minor < original_minor
