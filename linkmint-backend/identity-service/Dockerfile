# identity-service image. Runs migrations on start, then serves.
#
# Build context = repo ROOT (work15: installs the shared eventbus-python lib so the kafka
# outbox-drain relay + bus consumer work; lazily imported, only when IDENTITY_EVENT_PUBLISHER_MODE=
# kafka / IDENTITY_EVENT_CONSUMER_ENABLED=true). docker-compose sets `build.context: .` with this
# dockerfile path; by hand: `docker build -f linkmint-backend/identity-service/Dockerfile .` from root.
FROM python:3.12-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
RUN pip install --upgrade pip

# work15 — the shared Kafka client lib (used only in kafka mode / when the consumer is enabled;
# lazily imported).
COPY linkmint-backend/eventbus-python /eventbus-python
RUN pip install /eventbus-python

# work17 — the shared idempotency lib (Idempotency-Key store). Eagerly imported by the app
# (error handler + endpoints), so it is always installed (not optional like the bus client).
COPY linkmint-backend/idempotency-python /idempotency-python
RUN pip install /idempotency-python

# work18 — the shared telemetry lib (OTel init + ASGI middleware). Eagerly imported by app.main.
COPY linkmint-backend/telemetry-python /telemetry-python
RUN pip install /telemetry-python

# Service source (the package build reads pyproject + app/), then install.
COPY linkmint-backend/identity-service/pyproject.toml linkmint-backend/identity-service/alembic.ini linkmint-backend/identity-service/entrypoint.sh ./
COPY linkmint-backend/identity-service/app ./app
COPY linkmint-backend/identity-service/alembic ./alembic
RUN pip install . && chmod +x entrypoint.sh

# Run as non-root.
RUN useradd --create-home --uid 10001 appuser
USER appuser

EXPOSE 8090

HEALTHCHECK --interval=10s --timeout=3s --start-period=25s --retries=5 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8090/internal/healthz').status==200 else 1)"

ENTRYPOINT ["./entrypoint.sh"]
