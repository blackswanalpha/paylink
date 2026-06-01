#!/usr/bin/env sh
# Apply DB migrations, then serve the web API. Idempotent on restart.
set -e

echo "[entrypoint] applying migrations (alembic upgrade head)..."
alembic upgrade head

echo "[entrypoint] starting uvicorn..."
exec uvicorn app.main:app \
    --host "${NOTIFY_HTTP_HOST:-0.0.0.0}" \
    --port "${NOTIFY_HTTP_PORT:-8095}"
