# admin-backoffice image. Runs migrations on start, then serves.
#
# Build context = repo ROOT (work18: installs the shared telemetry-python lib, which app.main eagerly
# imports for OTel init + the ASGI observability middleware). docker-compose sets `build.context: .`
# with this dockerfile path; by hand: `docker build -f linkmint-backend/admin-backoffice/Dockerfile .`
FROM python:3.12-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
RUN pip install --upgrade pip

# work18 — the shared telemetry lib (OTel init + ASGI middleware). Eagerly imported by app.main.
COPY linkmint-backend/telemetry-python /telemetry-python
RUN pip install /telemetry-python

# Service source (the package build reads pyproject + app/), then install.
COPY linkmint-backend/admin-backoffice/pyproject.toml linkmint-backend/admin-backoffice/alembic.ini linkmint-backend/admin-backoffice/entrypoint.sh ./
COPY linkmint-backend/admin-backoffice/app ./app
COPY linkmint-backend/admin-backoffice/alembic ./alembic
RUN pip install . && chmod +x entrypoint.sh

# Run as non-root.
RUN useradd --create-home --uid 10001 appuser
USER appuser

EXPOSE 8092

HEALTHCHECK --interval=10s --timeout=3s --start-period=25s --retries=5 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8092/internal/healthz').status==200 else 1)"

ENTRYPOINT ["./entrypoint.sh"]
