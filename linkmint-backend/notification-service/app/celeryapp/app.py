"""Celery application + the worker-process delivery runner singleton.

The web app never imports this — only the worker (``celery -A app.celeryapp.app:celery_app worker``)
and tests (which flip ``task_always_eager``). The runner is built lazily, once per worker process,
with a SYNC engine + SYNC httpx client (Celery tasks are synchronous).
"""

from __future__ import annotations

import httpx
from celery import Celery

from app.channels.registry import build_channel_registry
from app.config import Settings, get_settings
from app.db.repository import SyncDeliveryStore
from app.db.session import make_sync_engine
from app.delivery.runner import DeliveryRunner
from app.logging import configure_logging


def make_celery(settings: Settings) -> Celery:
    celery = Celery(
        "notification",
        broker=settings.celery_broker_url,
        backend=settings.celery_result_backend,
    )
    celery.conf.task_always_eager = settings.celery_task_always_eager
    celery.conf.task_eager_propagates = True
    celery.conf.task_acks_late = True
    celery.conf.worker_prefetch_multiplier = 1
    celery.conf.task_default_queue = "notify"
    return celery


celery_app = make_celery(get_settings())

_runner: DeliveryRunner | None = None


def get_runner() -> DeliveryRunner:
    """Lazily build (and cache) the worker's DeliveryRunner."""
    global _runner
    if _runner is None:
        settings = get_settings()
        configure_logging(settings.log_level, settings.service_name)
        engine = make_sync_engine(settings.database_url)
        store = SyncDeliveryStore(engine)
        client = httpx.Client(timeout=10.0)
        channels = build_channel_registry(settings, client)
        _runner = DeliveryRunner(store, channels)
    return _runner


# Register the task on the app (must run after celery_app + get_runner are defined).
import app.celeryapp.tasks  # noqa: E402,F401
