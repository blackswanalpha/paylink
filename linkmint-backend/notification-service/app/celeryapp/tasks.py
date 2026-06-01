"""The thin Celery task — the only Celery-typed surface.

Delegates to the pure :class:`~app.delivery.runner.DeliveryRunner` and turns its ``should_retry`` /
``countdown`` into ``self.retry(countdown=...)``. The runner has already persisted FAILED +
``next_retry_at`` before we signal a retry, so the DB is consistent regardless of broker behaviour.
"""

from __future__ import annotations

import uuid

from celery import Task

from app.celeryapp.app import celery_app, get_runner
from app.delivery.backoff import MAX_RETRIES


@celery_app.task(bind=True, max_retries=MAX_RETRIES, name="notify.deliver")
def deliver(self: Task, delivery_id: str) -> str:
    outcome = get_runner().run_once(uuid.UUID(delivery_id))
    if outcome.should_retry and outcome.countdown is not None:
        raise self.retry(countdown=outcome.countdown)
    return outcome.status
