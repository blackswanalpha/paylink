# compliance-risk image. Runs migrations on start, then serves.
#
# Build context = repo ROOT (work17: installs the shared idempotency-python lib, which the app
# eagerly imports for the Idempotency-Key store + error handler). work15's shared eventbus-python
# lib is also installed for the outbox relay / bus consumer (lazily imported, kafka mode only).
# docker-compose sets `build.context: .` with this dockerfile path.
FROM python:3.12-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
RUN pip install --upgrade pip

# work15 — the shared Kafka client lib (used only by the outbox relay / bus consumer in kafka mode;
# lazily imported).
COPY linkmint-backend/eventbus-python /eventbus-python
RUN pip install /eventbus-python

# work17 — the shared idempotency lib (Idempotency-Key store + consumer dedupe). Eagerly imported by
# the app (error handler + endpoints), so it is always installed (not optional like the bus client).
COPY linkmint-backend/idempotency-python /idempotency-python
RUN pip install /idempotency-python

# work18 — the shared telemetry lib (OTel init + ASGI middleware). Eagerly imported by app.main.
COPY linkmint-backend/telemetry-python /telemetry-python
RUN pip install /telemetry-python

# Service source (the package build reads pyproject + app/), then install.
COPY linkmint-backend/compliance-risk/pyproject.toml linkmint-backend/compliance-risk/alembic.ini linkmint-backend/compliance-risk/entrypoint.sh ./
COPY linkmint-backend/compliance-risk/app ./app
COPY linkmint-backend/compliance-risk/alembic ./alembic
RUN pip install . && chmod +x entrypoint.sh

# Run as non-root.
RUN useradd --create-home --uid 10001 appuser
USER appuser

EXPOSE 8093

HEALTHCHECK --interval=10s --timeout=3s --start-period=25s --retries=5 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8093/internal/healthz').status==200 else 1)"

ENTRYPOINT ["./entrypoint.sh"]
