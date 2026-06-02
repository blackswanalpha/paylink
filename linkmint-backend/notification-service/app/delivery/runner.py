"""The delivery runner — the pure, unit-testable core of one delivery attempt.

NO Celery, NO SQLAlchemy imports: it takes a ``DeliveryStore`` (the durable row) and a
``ChannelLookup`` (the provider) behind Protocols, so the full QUEUED→SENT / →FAILED→…→EXHAUSTED
matrix + the exact backoff countdowns are tested with fakes — no broker, no DB, no network. The thin
Celery task in :mod:`app.celeryapp.tasks` wraps this and turns ``should_retry`` into ``self.retry``.

The row is the authoritative attempt counter (``attempts`` increments on failure only); the runner
persists FAILED + ``next_retry_at`` *before* signalling a retry, so the DB stays consistent even if
the broker later drops the in-flight task (a Phase-2 sweeper recovers it via the retry index).
"""

from __future__ import annotations

import uuid
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Protocol

from app.channels.base import Provider, SendError
from app.delivery.backoff import next_countdown
from app.domain.models import EXHAUSTED, FAILED, SENT, DeliveryRecord
from app.logging import get_logger
from app.redaction import mask_recipient

log = get_logger("notify.delivery")

NOOP = "NOOP"


@dataclass(frozen=True)
class RunOutcome:
    delivery_id: uuid.UUID
    status: str
    should_retry: bool
    countdown: int | None


class DeliveryStore(Protocol):
    def get(self, delivery_id: uuid.UUID) -> DeliveryRecord | None: ...
    def mark_sent(self, delivery_id: uuid.UUID, *, provider_ref: str) -> None: ...
    def mark_failed(
        self,
        delivery_id: uuid.UUID,
        *,
        attempts: int,
        last_error: str,
        next_retry_at: datetime,
    ) -> None: ...
    def mark_exhausted(self, delivery_id: uuid.UUID, *, attempts: int, last_error: str) -> None: ...


class ChannelLookup(Protocol):
    def for_channel(self, channel: str) -> Provider | None: ...


def _utcnow() -> datetime:
    return datetime.now(UTC)


class DeliveryRunner:
    def __init__(
        self,
        store: DeliveryStore,
        channels: ChannelLookup,
        clock: Callable[[], datetime] = _utcnow,
    ) -> None:
        self._store = store
        self._channels = channels
        self._clock = clock

    def run_once(self, delivery_id: uuid.UUID) -> RunOutcome:
        rec = self._store.get(delivery_id)
        if rec is None:
            log.warning("delivery_missing", delivery_id=str(delivery_id))
            return RunOutcome(delivery_id, NOOP, False, None)
        if rec.status in (SENT, EXHAUSTED):
            return RunOutcome(delivery_id, NOOP, False, None)

        provider = self._channels.for_channel(rec.channel)
        if provider is None:
            self._store.mark_exhausted(
                delivery_id, attempts=rec.attempts, last_error=f"no provider for {rec.channel}"
            )
            return RunOutcome(delivery_id, EXHAUSTED, False, None)

        try:
            result = provider.send(to=rec.recipient, body=rec.body, subject=rec.subject)
        except SendError as exc:
            attempts = rec.attempts + 1
            countdown = next_countdown(attempts)
            if countdown is None:
                self._store.mark_exhausted(delivery_id, attempts=attempts, last_error=str(exc))
                log.warning(
                    "delivery_exhausted",
                    delivery_id=str(delivery_id),
                    channel=rec.channel,
                    to=mask_recipient(rec.channel, rec.recipient),
                    attempts=attempts,
                )
                return RunOutcome(delivery_id, EXHAUSTED, False, None)
            next_retry_at = self._clock() + timedelta(seconds=countdown)
            self._store.mark_failed(
                delivery_id,
                attempts=attempts,
                last_error=str(exc),
                next_retry_at=next_retry_at,
            )
            log.info(
                "delivery_failed_retry",
                delivery_id=str(delivery_id),
                channel=rec.channel,
                to=mask_recipient(rec.channel, rec.recipient),
                attempts=attempts,
                countdown=countdown,
            )
            return RunOutcome(delivery_id, FAILED, True, countdown)

        self._store.mark_sent(delivery_id, provider_ref=result.provider_ref)
        log.info(
            "delivery_sent",
            delivery_id=str(delivery_id),
            channel=rec.channel,
            to=mask_recipient(rec.channel, rec.recipient),
            provider=result.provider,
            provider_ref=result.provider_ref,
        )
        return RunOutcome(delivery_id, SENT, False, None)
