from __future__ import annotations

import uuid
from decimal import Decimal

from app.domain.models import FlagKind
from app.domain.risk_engine import Decision, Reason
from app.domain.risk_service import RiskService, _dominant_kind
from app.events import publisher as ev
from app.events.stub import NoopPublisher
from tests._support import FakeRepository, make_settings, noop_commit


def _svc(repo: FakeRepository, **overrides: object) -> RiskService:
    settings = make_settings(**overrides)
    return RiskService(repo, noop_commit, NoopPublisher(), settings)  # type: ignore[arg-type]


# ── dominant kind (pure helper) ──
def test_dominant_kind_prefers_highest_weight() -> None:
    reasons = [
        Reason("GEO_MISMATCH", "geo", 0.3),
        Reason("AML_THRESHOLD", "aml", 1.0),
    ]
    assert _dominant_kind(reasons) is FlagKind.VELOCITY  # AML maps to velocity, higher weight


def test_dominant_kind_geo() -> None:
    assert _dominant_kind([Reason("GEO_MISMATCH", "geo", 0.3)]) is FlagKind.GEO


def test_dominant_kind_defaults_manual() -> None:
    # LOW_KYC has no mapping → MANUAL.
    assert _dominant_kind([Reason("LOW_KYC", "x", 1.0)]) is FlagKind.MANUAL
    assert _dominant_kind([]) is FlagKind.MANUAL


# ── evaluate persistence + events ──
async def test_evaluate_allow_persists_score_and_check_passed() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    out = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(10),
        currency="KES",
        geo_country=None,
        registered_country=None,
    )
    assert out.decision is Decision.ALLOW
    assert len(repo.risk_scores) == 1 and repo.risk_scores[0].decision == "allow"
    assert repo.flags == []
    kinds = [k for (_, _, k, _) in repo.events]
    assert ev.CHECK_PASSED in kinds and ev.CHECK_FAILED not in kinds


async def test_evaluate_block_raises_block_flag_and_two_events() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()  # tier 0
    out = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(100),
        currency="KES",
        geo_country=None,
        registered_country=None,
    )
    assert out.decision is Decision.BLOCK
    assert repo.flags[0].severity == "block"
    kinds = [k for (_, _, k, _) in repo.events]
    assert ev.CHECK_FAILED in kinds and ev.FLAG_RAISED in kinds


async def test_evaluate_review_raises_warn_flag() -> None:
    from datetime import UTC, datetime, timedelta

    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    # geo-only is 0.3 → allow.
    out = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(10),
        currency="KES",
        geo_country="NG",
        registered_country="KE",
    )
    assert out.decision is Decision.ALLOW
    # 20 events 2h ago: inside the 24h window, OUTSIDE the 1h window → VELOCITY_24H (0.4) only.
    repo.seed_activity(uid, n=20, when=datetime.now(UTC) - timedelta(hours=2))
    out2 = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(10),
        currency="KES",
        geo_country="NG",
        registered_country="KE",
    )
    # 0.4 (velocity24) + 0.3 (geo) = 0.7 ≥ 0.5 → review; dominant reason is velocity (0.4 > 0.3).
    assert out2.decision is Decision.REVIEW
    assert out2.score == 0.7
    warn_flags = [f for f in repo.flags if f.severity == "warn"]
    assert warn_flags and warn_flags[-1].kind == "velocity"


async def test_evaluate_uses_context_when_provided() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(10),
        currency="KES",
        geo_country=None,
        registered_country=None,
        context="paylink.create:PLK-1",
    )
    assert repo.risk_scores[0].context == "paylink.create:PLK-1"


async def test_evaluate_reads_velocity_from_activity() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=2)
    repo.seed_activity(uid, n=50)  # ≥ block_24h
    out = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(10),
        currency="KES",
        geo_country=None,
        registered_country=None,
    )
    assert "VELOCITY_24H" in {r.code for r in out.reasons}
    assert out.decision is Decision.BLOCK  # score 0.8 ≥ block threshold


async def test_evaluate_aml_reads_cumulative_sum_from_activity() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    repo.seed_kyc(uid, tier=1)
    repo.seed_activity(uid, n=1, amount=Decimal(120000))  # prior cumulative
    out = await _svc(repo).evaluate(
        user_id=uid,
        action="paylink.create",
        amount=Decimal(40000),  # 120k + 40k = 160k ≥ 150k, tier 1 → block
        currency="KES",
        geo_country=None,
        registered_country=None,
    )
    assert "AML_THRESHOLD" in {r.code for r in out.reasons}
    assert out.decision is Decision.BLOCK


async def test_record_activity_appends_and_does_not_touch_score() -> None:
    repo = FakeRepository()
    uid = uuid.uuid4()
    await _svc(repo).record_activity(
        user_id=uid, action="payment.initiated", amount=Decimal("99.99"), currency="KES"
    )
    assert len(repo.activity) == 1
    assert repo.activity[0].amount == Decimal("99.99")
    assert repo.risk_scores == []  # recording activity never writes a risk score
