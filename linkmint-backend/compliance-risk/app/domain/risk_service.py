"""Risk evaluation orchestration (I/O around the pure engine).

``evaluate`` resolves the engine inputs from the repo (tier from ``kyc_records``; velocity counts +
the cumulative-amount window from ``activity_events``), calls the pure :func:`risk_engine.evaluate`,
persists a ``risk_scores`` row, and — per decision — raises flags + writes outbox events, then
commits and publishes post-commit (the reference outbox pattern: mutate + ``add_event`` in one
transaction → ``commit`` → ``publish``):

- ``block``  → a ``block`` flag (kind from the dominant reason) + ``compliance.check.failed`` +
  ``compliance.flag.raised``.
- ``review`` → a ``warn`` flag + ``compliance.flag.raised``.
- ``allow``  → ``compliance.check.passed``.

``evaluate`` deliberately does NOT append an ``activity_event`` (so a risk read never pollutes the
velocity windows); the ``payment.initiated`` consumer feeds activity via :meth:`record_activity`.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime, timedelta
from decimal import Decimal

from app.config import Settings
from app.db.models import ActivityEventRow, FlagRow, RiskScoreRow
from app.db.repositories import ComplianceRepository
from app.domain.models import FlagKind, FlagSeverity
from app.domain.risk_engine import Decision, Reason, RiskInputs, RiskOutcome, evaluate
from app.events import publisher as events
from app.events.publisher import Publisher

_Commit = Callable[[], Awaitable[None]]

# Map the dominant reason code → the flag ``kind`` recorded for the block/warn flag.
_KIND_BY_REASON: dict[str, FlagKind] = {
    "AML_THRESHOLD": FlagKind.VELOCITY,
    "VELOCITY_24H": FlagKind.VELOCITY,
    "VELOCITY_1H": FlagKind.VELOCITY,
    "GEO_MISMATCH": FlagKind.GEO,
}


def _reasons_payload(reasons: list[Reason]) -> list[dict[str, object]]:
    return [{"code": r.code, "detail": r.detail} for r in reasons]


def _dominant_kind(reasons: list[Reason]) -> FlagKind:
    """The flag kind from the highest-weight reason (ties keep the first); default MANUAL."""
    for reason in sorted(reasons, key=lambda r: r.weight, reverse=True):
        kind = _KIND_BY_REASON.get(reason.code)
        if kind is not None:
            return kind
    return FlagKind.MANUAL


class RiskService:
    def __init__(
        self,
        repo: ComplianceRepository,
        commit: _Commit,
        publisher: Publisher,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher
        self._settings = settings

    async def evaluate(
        self,
        *,
        user_id: uuid.UUID,
        action: str,
        amount: Decimal | None,
        currency: str,
        geo_country: str | None,
        registered_country: str | None,
        context: str | None = None,
    ) -> RiskOutcome:
        now = datetime.now(UTC)
        tier = await self._repo.get_tier(user_id)
        count_1h = await self._repo.count_activity_since(user_id, now - timedelta(hours=1))
        count_24h = await self._repo.count_activity_since(user_id, now - timedelta(hours=24))
        count_7d = await self._repo.count_activity_since(user_id, now - timedelta(days=7))
        aml_since = now - timedelta(hours=self._settings.aml_window_hours)
        cumulative = await self._repo.sum_amount_since(user_id, aml_since)

        inputs = RiskInputs(
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
        outcome = evaluate(inputs, self._settings.risk_config())

        ctx = context or action
        await self._repo.insert_risk_score(
            RiskScoreRow(
                user_id=user_id,
                context=ctx,
                score=Decimal(str(outcome.score)),
                decision=outcome.decision.value,
                reasons=_reasons_payload(outcome.reasons),
            )
        )

        check_payload: dict[str, object] = {
            "user_id": str(user_id),
            "action": action,
            "decision": outcome.decision.value,
            "score": outcome.score,
            "reasons": _reasons_payload(outcome.reasons),
        }
        post_commit: list[tuple[str, dict[str, object]]] = []

        if outcome.decision is Decision.BLOCK:
            flag_payload = await self._raise_flag(user_id, FlagSeverity.BLOCK, outcome, action)
            await self._repo.add_event("risk", user_id, events.CHECK_FAILED, check_payload)
            await self._repo.add_event("flag", user_id, events.FLAG_RAISED, flag_payload)
            post_commit.append((events.CHECK_FAILED, check_payload))
            post_commit.append((events.FLAG_RAISED, flag_payload))
        elif outcome.decision is Decision.REVIEW:
            flag_payload = await self._raise_flag(user_id, FlagSeverity.WARN, outcome, action)
            await self._repo.add_event("flag", user_id, events.FLAG_RAISED, flag_payload)
            post_commit.append((events.FLAG_RAISED, flag_payload))
        else:
            await self._repo.add_event("risk", user_id, events.CHECK_PASSED, check_payload)
            post_commit.append((events.CHECK_PASSED, check_payload))

        await self._commit()
        for name, payload in post_commit:
            await self._publisher.publish(name, payload)
        return outcome

    async def _raise_flag(
        self,
        user_id: uuid.UUID,
        severity: FlagSeverity,
        outcome: RiskOutcome,
        action: str,
    ) -> dict[str, object]:
        kind = _dominant_kind(outcome.reasons)
        payload: dict[str, object] = {
            "user_id": str(user_id),
            "kind": kind.value,
            "severity": severity.value,
            "action": action,
            "decision": outcome.decision.value,
            "score": outcome.score,
            "reasons": _reasons_payload(outcome.reasons),
        }
        await self._repo.insert_flag(
            FlagRow(
                user_id=user_id,
                kind=kind.value,
                severity=severity.value,
                payload=payload,
            )
        )
        return payload

    async def record_activity(
        self,
        *,
        user_id: uuid.UUID,
        action: str,
        amount: Decimal | None,
        currency: str | None,
    ) -> None:
        """Append an activity event (the velocity/AML signal) and commit. Used by the consumer."""
        await self._repo.insert_activity(
            ActivityEventRow(
                user_id=user_id,
                action=action,
                amount=amount,
                currency=currency,
            )
        )
        await self._commit()
