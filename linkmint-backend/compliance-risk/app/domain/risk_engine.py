"""The risk engine — a PURE, deterministic decision function (NO I/O).

``evaluate(RiskInputs, RiskConfig) -> RiskOutcome`` applies hard rules first (each contributing a
reason with a weight that feeds the score AND can independently force a block/review), then weighted
soft signals, then combines them into a final ``{decision, score 0..1 (3dp), reasons[]}``.

Determinism is an acceptance criterion (a fixed fixture → a fixed decision), which is why this layer
has no clock, DB, or randomness — the windowed counts/sums are computed by :class:`RiskService` and
passed in. Thresholds all come from :class:`RiskConfig` (built from ``Settings``), so policy is
env-tunable without touching this code.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal
from enum import StrEnum

# Actions that move value (and therefore trip the KYC/AML/ceiling hard rules). A non-value action
# (e.g. a profile read) never hard-blocks on amount, regardless of tier.
VALUE_ACTIONS: frozenset[str] = frozenset({"paylink.create", "payment.initiate", "withdraw"})


class Decision(StrEnum):
    ALLOW = "allow"
    BLOCK = "block"
    REVIEW = "review"


@dataclass(frozen=True)
class RiskConfig:
    """Engine policy (env-tunable). ``tier_ceilings`` maps a KYC tier → its per-tx ceiling."""

    tier_ceilings: dict[int, Decimal]
    aml_cumulative_threshold: Decimal
    velocity_block_24h: int
    velocity_review_24h: int
    velocity_review_1h: int
    score_block_threshold: float
    score_review_threshold: float


@dataclass(frozen=True)
class RiskInputs:
    """Everything the engine needs, resolved by :class:`RiskService` (no I/O happens in here)."""

    tier: int
    action: str
    amount: Decimal | None
    currency: str
    geo_country: str | None
    registered_country: str | None
    count_1h: int
    count_24h: int
    count_7d: int
    cumulative_amount_window: Decimal


@dataclass(frozen=True)
class Reason:
    code: str
    detail: str
    weight: float


@dataclass(frozen=True)
class RiskOutcome:
    decision: Decision
    score: float
    reasons: list[Reason] = field(default_factory=list)


def _ceiling_for(cfg: RiskConfig, tier: int) -> Decimal:
    """The per-tx amount ceiling for ``tier`` (unknown tiers fall back to the tier-0 ceiling)."""
    return cfg.tier_ceilings.get(tier, cfg.tier_ceilings.get(0, Decimal(0)))


def evaluate(inputs: RiskInputs, cfg: RiskConfig) -> RiskOutcome:
    """Score the action and decide allow/block/review. Pure + deterministic."""
    reasons: list[Reason] = []
    hard_block = False
    hard_review = False

    amount = inputs.amount
    is_value_action = inputs.action in VALUE_ACTIONS
    has_amount = amount is not None and amount > 0

    # ── HARD RULES (each adds a weighted reason AND can force a decision) ──

    # LOW_KYC: a tier-0 user attempting a value action with money on the line.
    if is_value_action and inputs.tier <= 0 and has_amount:
        reasons.append(
            Reason(
                code="LOW_KYC",
                detail="tier-0 user cannot perform a value action with a non-zero amount",
                weight=1.0,
            )
        )
        hard_block = True

    # AML_THRESHOLD (Kenya): cumulative window + this amount crosses the AML ceiling without
    # enhanced (tier-2) KYC. tier<=1 → block; tier==2 is cleared (enhanced KYC); a tier in (1,2)
    # is treated as a hard review (defensive — there is no such integer tier today).
    if is_value_action:
        projected = inputs.cumulative_amount_window + (amount or Decimal(0))
        if projected >= cfg.aml_cumulative_threshold and inputs.tier < 2:
            reasons.append(
                Reason(
                    code="AML_THRESHOLD",
                    detail=(
                        "cumulative amount over the window crosses the Kenya AML threshold "
                        "without enhanced (tier-2) KYC"
                    ),
                    weight=1.0,
                )
            )
            if inputs.tier <= 1:
                hard_block = True
            else:
                hard_review = True

    # AMOUNT_OVER_TIER_CEILING: a single amount above the tier's per-tx ceiling.
    if is_value_action and amount is not None:
        ceiling = _ceiling_for(cfg, inputs.tier)
        if amount > ceiling:
            reasons.append(
                Reason(
                    code="AMOUNT_OVER_TIER_CEILING",
                    detail=f"amount exceeds the tier-{inputs.tier} per-transaction ceiling",
                    weight=0.6,
                )
            )
            hard_review = True

    # ── SOFT SIGNALS (accumulate into the score; the strongest velocity tier can force a review) ──
    soft_score = 0.0

    if inputs.count_24h >= cfg.velocity_block_24h:
        soft_score += 0.8
        hard_review = True
        reasons.append(
            Reason(
                code="VELOCITY_24H",
                detail=f"{inputs.count_24h} actions in 24h exceeds the block threshold",
                weight=0.8,
            )
        )
    elif inputs.count_24h >= cfg.velocity_review_24h:
        soft_score += 0.4
        reasons.append(
            Reason(
                code="VELOCITY_24H",
                detail=f"{inputs.count_24h} actions in 24h exceeds the review threshold",
                weight=0.4,
            )
        )

    if inputs.count_1h >= cfg.velocity_review_1h:
        soft_score += 0.3
        reasons.append(
            Reason(
                code="VELOCITY_1H",
                detail=f"{inputs.count_1h} actions in 1h exceeds the review threshold",
                weight=0.3,
            )
        )

    if (
        inputs.geo_country
        and inputs.registered_country
        and inputs.geo_country.upper() != inputs.registered_country.upper()
    ):
        soft_score += 0.3
        reasons.append(
            Reason(
                code="GEO_MISMATCH",
                detail="geo-IP country differs from the registered country",
                weight=0.3,
            )
        )

    # ── COMBINE ──
    hard_weight = sum(r.weight for r in reasons if r.code in _HARD_CODES)
    score = round(min(1.0, soft_score + hard_weight), 3)

    if hard_block or score >= cfg.score_block_threshold:
        decision = Decision.BLOCK
    elif hard_review or score >= cfg.score_review_threshold:
        decision = Decision.REVIEW
    else:
        decision = Decision.ALLOW

    return RiskOutcome(decision=decision, score=score, reasons=reasons)


# Hard-rule reason codes whose weights add to the score (soft signals sum via ``soft_score``).
_HARD_CODES: frozenset[str] = frozenset({"LOW_KYC", "AML_THRESHOLD", "AMOUNT_OVER_TIER_CEILING"})
