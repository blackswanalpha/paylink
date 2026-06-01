"""Table-driven tests for the pure risk engine — every decision branch (the coverage workhorse).

Determinism is an acceptance criterion: a fixed fixture → a fixed decision. ``_cfg`` mirrors the
default ``Settings.risk_config()`` thresholds.
"""

from __future__ import annotations

from decimal import Decimal

import pytest

from app.domain.risk_engine import (
    VALUE_ACTIONS,
    Decision,
    RiskConfig,
    RiskInputs,
    evaluate,
)


def _cfg() -> RiskConfig:
    return RiskConfig(
        tier_ceilings={0: Decimal(0), 1: Decimal(50000), 2: Decimal("Infinity")},
        aml_cumulative_threshold=Decimal(150000),
        velocity_block_24h=50,
        velocity_review_24h=20,
        velocity_review_1h=10,
        score_block_threshold=0.8,
        score_review_threshold=0.5,
    )


def _inputs(
    *,
    tier: int = 2,
    action: str = "paylink.create",
    amount: Decimal | None = Decimal(1000),
    currency: str = "KES",
    geo_country: str | None = None,
    registered_country: str | None = None,
    count_1h: int = 0,
    count_24h: int = 0,
    count_7d: int = 0,
    cumulative: Decimal = Decimal(0),
) -> RiskInputs:
    return RiskInputs(
        tier=tier,
        action=action,
        amount=amount,
        currency=currency,
        geo_country=geo_country,
        registered_country=registered_country,
        count_1h=count_1h,
        count_24h=count_24h,
        count_7d=count_7d,
        cumulative_amount_window=cumulative,
    )


def _codes(inputs: RiskInputs) -> set[str]:
    return {r.code for r in evaluate(inputs, _cfg()).reasons}


# ── allow baselines ──
def test_clean_tier2_small_amount_allows() -> None:
    out = evaluate(_inputs(tier=2, amount=Decimal(1000)), _cfg())
    assert out.decision is Decision.ALLOW
    assert out.score == 0.0
    assert out.reasons == []


def test_non_value_action_never_hard_blocks_even_at_tier0() -> None:
    # A non-value action with a tier-0 user + amount must not trip LOW_KYC/AML/ceiling.
    out = evaluate(_inputs(tier=0, action="profile.read", amount=Decimal(999999)), _cfg())
    assert out.decision is Decision.ALLOW
    assert out.reasons == []


def test_tier1_amount_at_ceiling_allows() -> None:
    # Exactly at the ceiling (50000) is NOT over it.
    out = evaluate(_inputs(tier=1, amount=Decimal(50000)), _cfg())
    assert out.decision is Decision.ALLOW


def test_zero_amount_value_action_tier0_allows() -> None:
    # amount==0 is not "has_amount", so LOW_KYC does not fire.
    out = evaluate(_inputs(tier=0, amount=Decimal(0)), _cfg())
    assert out.decision is Decision.ALLOW


def test_none_amount_value_action_tier0_allows() -> None:
    out = evaluate(_inputs(tier=0, amount=None), _cfg())
    assert out.decision is Decision.ALLOW


# ── LOW_KYC (hard block) ──
def test_tier0_value_action_with_amount_blocks() -> None:
    out = evaluate(_inputs(tier=0, amount=Decimal(100)), _cfg())
    assert out.decision is Decision.BLOCK
    assert "LOW_KYC" in {r.code for r in out.reasons}
    assert out.score == 1.0


@pytest.mark.parametrize("action", sorted(VALUE_ACTIONS))
def test_every_value_action_low_kyc_blocks(action: str) -> None:
    out = evaluate(_inputs(tier=0, action=action, amount=Decimal(5)), _cfg())
    assert out.decision is Decision.BLOCK


# ── AML_THRESHOLD ──
def test_aml_tier1_over_threshold_blocks() -> None:
    # cumulative 100k + amount 60k = 160k >= 150k, tier 1 → block.
    out = evaluate(_inputs(tier=1, amount=Decimal(60000), cumulative=Decimal(100000)), _cfg())
    assert out.decision is Decision.BLOCK
    assert "AML_THRESHOLD" in {r.code for r in out.reasons}


def test_aml_tier2_over_threshold_allows() -> None:
    # tier 2 (enhanced KYC) clears the AML threshold entirely.
    out = evaluate(_inputs(tier=2, amount=Decimal(200000), cumulative=Decimal(0)), _cfg())
    assert out.decision is Decision.ALLOW
    assert "AML_THRESHOLD" not in {r.code for r in out.reasons}


def test_aml_exactly_at_threshold_blocks_tier1() -> None:
    # projected == threshold (>=) trips the rule.
    out = evaluate(_inputs(tier=1, amount=Decimal(150000), cumulative=Decimal(0)), _cfg())
    assert "AML_THRESHOLD" in {r.code for r in out.reasons}
    assert out.decision is Decision.BLOCK


def test_aml_tier0_blocks_via_low_kyc_and_aml() -> None:
    out = evaluate(_inputs(tier=0, amount=Decimal(200000), cumulative=Decimal(0)), _cfg())
    codes = {r.code for r in out.reasons}
    assert "LOW_KYC" in codes and "AML_THRESHOLD" in codes
    assert out.decision is Decision.BLOCK


# ── AMOUNT_OVER_TIER_CEILING (hard review) ──
def test_amount_over_tier1_ceiling_reviews() -> None:
    # tier 1 ceiling is 50000; 60000 is over but below the AML threshold → review (not block).
    out = evaluate(_inputs(tier=1, amount=Decimal(60000), cumulative=Decimal(0)), _cfg())
    assert out.decision is Decision.REVIEW
    assert "AMOUNT_OVER_TIER_CEILING" in {r.code for r in out.reasons}


def test_amount_over_tier0_ceiling_blocks_via_low_kyc() -> None:
    # tier 0 ceiling is 0, so any amount > 0 is over — but LOW_KYC already forces a block.
    out = evaluate(_inputs(tier=0, amount=Decimal(10)), _cfg())
    assert out.decision is Decision.BLOCK
    codes = {r.code for r in out.reasons}
    assert "LOW_KYC" in codes and "AMOUNT_OVER_TIER_CEILING" in codes


def test_unknown_tier_falls_back_to_tier0_ceiling() -> None:
    # tier 5 is unknown → ceiling falls back to tier-0 (0). amount>0 → over ceiling → review,
    # and tier<2 with a big amount also trips AML → block.
    out = evaluate(_inputs(tier=5, amount=Decimal(10), cumulative=Decimal(0)), _cfg())
    assert "AMOUNT_OVER_TIER_CEILING" in {r.code for r in out.reasons}


# ── VELOCITY signals ──
def test_velocity_24h_block_threshold_reviews_and_scores() -> None:
    # 50 in 24h → +0.8 soft + hard_review. No hard block (score 0.8 alone == block threshold).
    out = evaluate(_inputs(tier=2, amount=Decimal(10), count_24h=50), _cfg())
    assert "VELOCITY_24H" in {r.code for r in out.reasons}
    # score 0.8 >= block threshold (0.8) → BLOCK by score.
    assert out.score == 0.8
    assert out.decision is Decision.BLOCK


def test_velocity_24h_review_threshold_is_soft_bump_only() -> None:
    # The review-tier velocity (>=20) is a SOFT 0.4 signal, not a hard review — 0.4 < 0.5 → allow.
    out = evaluate(_inputs(tier=2, amount=Decimal(10), count_24h=20), _cfg())
    assert out.score == 0.4
    assert "VELOCITY_24H" in {r.code for r in out.reasons}
    assert out.decision is Decision.ALLOW


def test_velocity_24h_just_below_review_allows() -> None:
    out = evaluate(_inputs(tier=2, amount=Decimal(10), count_24h=19), _cfg())
    assert out.decision is Decision.ALLOW
    assert "VELOCITY_24H" not in {r.code for r in out.reasons}


def test_velocity_1h_bump_reviews_when_combined() -> None:
    # 20 in 24h (+0.4) and 10 in 1h (+0.3) → 0.7 → review.
    out = evaluate(_inputs(tier=2, amount=Decimal(10), count_24h=20, count_1h=10), _cfg())
    assert out.score == 0.7
    assert out.decision is Decision.REVIEW
    assert {"VELOCITY_24H", "VELOCITY_1H"} <= {r.code for r in out.reasons}


def test_velocity_1h_alone_below_review_allows() -> None:
    # 10 in 1h alone is +0.3 → below the 0.5 review threshold → allow.
    out = evaluate(_inputs(tier=2, amount=Decimal(10), count_1h=10), _cfg())
    assert out.score == 0.3
    assert out.decision is Decision.ALLOW


# ── GEO_MISMATCH ──
def test_geo_mismatch_alone_allows_but_flags() -> None:
    out = evaluate(
        _inputs(tier=2, amount=Decimal(10), geo_country="NG", registered_country="KE"),
        _cfg(),
    )
    assert "GEO_MISMATCH" in {r.code for r in out.reasons}
    assert out.score == 0.3
    assert out.decision is Decision.ALLOW


def test_geo_match_no_flag() -> None:
    out = evaluate(
        _inputs(tier=2, amount=Decimal(10), geo_country="ke", registered_country="KE"),
        _cfg(),
    )
    assert "GEO_MISMATCH" not in {r.code for r in out.reasons}


def test_geo_missing_one_side_no_flag() -> None:
    assert "GEO_MISMATCH" not in _codes(
        _inputs(tier=2, amount=Decimal(10), geo_country=None, registered_country="KE")
    )
    assert "GEO_MISMATCH" not in _codes(
        _inputs(tier=2, amount=Decimal(10), geo_country="NG", registered_country=None)
    )


# ── score-boundary rounding + combine ──
def test_score_review_boundary_geo_plus_velocity24() -> None:
    # 20 in 24h (+0.4) + geo mismatch (+0.3) = 0.7 → review.
    out = evaluate(
        _inputs(
            tier=2,
            amount=Decimal(10),
            count_24h=20,
            geo_country="NG",
            registered_country="KE",
        ),
        _cfg(),
    )
    assert out.score == 0.7
    assert out.decision is Decision.REVIEW


def test_score_block_by_accumulated_softsignals() -> None:
    # velocity24 block (0.8) + 1h (0.3) + geo (0.3) clamps to 1.0 → block by score.
    out = evaluate(
        _inputs(
            tier=2,
            amount=Decimal(10),
            count_24h=50,
            count_1h=10,
            geo_country="NG",
            registered_country="KE",
        ),
        _cfg(),
    )
    assert out.score == 1.0
    assert out.decision is Decision.BLOCK


def test_score_is_3dp_rounded() -> None:
    # A single 0.3 soft signal stays exact; assert the score is a clean float at 3dp.
    out = evaluate(
        _inputs(tier=2, amount=Decimal(10), geo_country="NG", registered_country="KE"),
        _cfg(),
    )
    assert out.score == round(out.score, 3)


def test_determinism_same_inputs_same_outcome() -> None:
    a = evaluate(_inputs(tier=1, amount=Decimal(60000), cumulative=Decimal(100000)), _cfg())
    b = evaluate(_inputs(tier=1, amount=Decimal(60000), cumulative=Decimal(100000)), _cfg())
    assert (a.decision, a.score) == (b.decision, b.score)
    assert [r.code for r in a.reasons] == [r.code for r in b.reasons]
