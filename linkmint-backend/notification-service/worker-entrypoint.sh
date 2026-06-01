#!/usr/bin/env sh
# Celery worker. The web container owns migrations (alembic upgrade head); the worker only processes
# delivery tasks, which are only enqueued once the web API is up — so no migration runs here.
set -e

echo "[worker-entrypoint] starting celery worker (queue: notify)..."
exec celery -A app.celeryapp.app:celery_app worker \
    --queues notify \
    --concurrency "${NOTIFY_WORKER_CONCURRENCY:-2}" \
    --loglevel "${NOTIFY_LOG_LEVEL:-INFO}"
