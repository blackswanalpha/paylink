"""Pure state-machine + value-helper tests."""

from __future__ import annotations

import pytest

from app.domain.models import (
    DISPUTE_LOSS_STATES,
    DisputeState,
    RefundState,
    dispute_can_transition,
    is_partial_refund,
    refund_can_transition,
)


@pytest.mark.parametrize(
    "current,target,ok",
    [
        (RefundState.REQUESTED, RefundState.PROCESSING, True),
        (RefundState.REQUESTED, RefundState.REJECTED, True),
        (RefundState.REQUESTED, RefundState.APPROVED, True),
        (RefundState.PROCESSING, RefundState.COMPLETED, True),
        (RefundState.PROCESSING, RefundState.FAILED, True),
        (RefundState.REQUESTED, RefundState.COMPLETED, False),
        (RefundState.COMPLETED, RefundState.PROCESSING, False),
        (RefundState.REJECTED, RefundState.PROCESSING, False),
        (RefundState.FAILED, RefundState.COMPLETED, False),
    ],
)
def test_refund_transitions(current: RefundState, target: RefundState, ok: bool) -> None:
    assert refund_can_transition(current, target) is ok


@pytest.mark.parametrize(
    "current,target,ok",
    [
        (DisputeState.OPEN, DisputeState.SUBMITTED, True),
        (DisputeState.OPEN, DisputeState.EXPIRED, True),
        (DisputeState.SUBMITTED, DisputeState.WON, True),
        (DisputeState.SUBMITTED, DisputeState.LOST, True),
        (DisputeState.WON, DisputeState.LOST, False),
        (DisputeState.LOST, DisputeState.WON, False),
        (DisputeState.SUBMITTED, DisputeState.OPEN, False),
        (DisputeState.EXPIRED, DisputeState.WON, False),
    ],
)
def test_dispute_transitions(current: DisputeState, target: DisputeState, ok: bool) -> None:
    assert dispute_can_transition(current, target) is ok


def test_loss_states() -> None:
    assert DisputeState.LOST in DISPUTE_LOSS_STATES
    assert DisputeState.EXPIRED in DISPUTE_LOSS_STATES
    assert DisputeState.WON not in DISPUTE_LOSS_STATES


def test_is_partial_refund() -> None:
    assert is_partial_refund(50, 100) is True
    assert is_partial_refund(100, 100) is False
    assert is_partial_refund(100, None) is True  # unknown original → conservatively partial
