from __future__ import annotations

import pytest

from app.domain import state_machine as sm
from app.domain.models import MerchantStatus, ReviewDecision
from app.errors import AppError, ErrorCode

S = MerchantStatus


def test_allowed_edges() -> None:
    assert sm.can_transition(S.DRAFT, S.PENDING_VERIFICATION)
    assert sm.can_transition(S.PENDING_VERIFICATION, S.ACTIVE)
    assert sm.can_transition(S.PENDING_VERIFICATION, S.REJECTED)
    assert sm.can_transition(S.PENDING_VERIFICATION, S.SUSPENDED)
    assert sm.can_transition(S.ACTIVE, S.SUSPENDED)
    assert sm.can_transition(S.SUSPENDED, S.ACTIVE)
    assert sm.can_transition(S.SUSPENDED, S.REJECTED)


def test_disallowed_edges() -> None:
    assert not sm.can_transition(S.DRAFT, S.ACTIVE)
    assert not sm.can_transition(S.ACTIVE, S.REJECTED)
    assert not sm.can_transition(S.ACTIVE, S.PENDING_VERIFICATION)
    assert not sm.can_transition(S.PENDING_VERIFICATION, S.DRAFT)


def test_rejected_is_terminal() -> None:
    for target in S:
        assert not sm.can_transition(S.REJECTED, target)


def test_target_for_decision() -> None:
    assert sm.target_for(ReviewDecision.APPROVE) == S.ACTIVE
    assert sm.target_for(ReviewDecision.REINSTATE) == S.ACTIVE
    assert sm.target_for(ReviewDecision.REJECT) == S.REJECTED
    assert sm.target_for(ReviewDecision.SUSPEND) == S.SUSPENDED


def test_assert_transition_ok() -> None:
    sm.assert_transition(S.PENDING_VERIFICATION, S.ACTIVE)  # no raise


def test_assert_transition_raises_invalid() -> None:
    with pytest.raises(AppError) as exc:
        sm.assert_transition(S.ACTIVE, S.REJECTED)
    assert exc.value.code == ErrorCode.INVALID_TRANSITION
    assert exc.value.http_status == 409
    assert exc.value.details == {"from": "ACTIVE", "to": "REJECTED"}
