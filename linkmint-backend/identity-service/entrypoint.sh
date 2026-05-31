#!/usr/bin/env sh
# Apply DB migrations, then serve. Idempotent on restart.
set -e

echo "[entrypoint] applying migrations (alembic upgrade head)..."
alembic upgrade head

echo "[entrypoint] starting uvicorn..."
exec uvicorn app.main:app \
    --host "${IDENTITY_HTTP_HOST:-0.0.0.0}" \
    --port "${IDENTITY_HTTP_PORT:-8090}"
