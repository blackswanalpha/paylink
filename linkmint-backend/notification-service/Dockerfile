# notification-service image. The web container runs migrations on start, then serves; the worker
# container runs `celery worker` (same image, worker-entrypoint.sh).
#
# Build context = repo ROOT (work15: installs the shared eventbus-python lib so the bus consumer
# works; lazily imported, only when NOTIFY_EVENT_CONSUMER_ENABLED=true). docker-compose sets
# `build.context: .` with this dockerfile path.
FROM python:3.12-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
RUN pip install --upgrade pip

# work15 — the shared Kafka client lib (used only when the bus consumer is enabled; lazily imported).
COPY linkmint-backend/eventbus-python /eventbus-python
RUN pip install /eventbus-python

# work17 — the shared idempotency lib (Idempotency-Key store + consumer dedupe). Eagerly imported by
# the app (error handler + endpoints), so it is always installed (not optional like the bus client).
COPY linkmint-backend/idempotency-python /idempotency-python
RUN pip install /idempotency-python

# work18 — the shared telemetry lib (OTel init + ASGI middleware + Celery trace helpers). Eagerly
# imported by app.main and the Celery task.
COPY linkmint-backend/telemetry-python /telemetry-python
RUN pip install /telemetry-python

# Service source (the package build reads pyproject + app/), then install.
COPY linkmint-backend/notification-service/pyproject.toml linkmint-backend/notification-service/alembic.ini linkmint-backend/notification-service/entrypoint.sh linkmint-backend/notification-service/worker-entrypoint.sh ./
COPY linkmint-backend/notification-service/app ./app
COPY linkmint-backend/notification-service/alembic ./alembic
RUN pip install . && chmod +x entrypoint.sh worker-entrypoint.sh

# Run as non-root.
RUN useradd --create-home --uid 10001 appuser
USER appuser

EXPOSE 8095

HEALTHCHECK --interval=10s --timeout=3s --start-period=25s --retries=5 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8095/internal/healthz').status==200 else 1)"

ENTRYPOINT ["./entrypoint.sh"]
