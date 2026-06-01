# merchant-onboarding image. Runs migrations on start, then serves.
FROM python:3.12-slim
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

# Source first (the package build reads pyproject + app/), then install.
COPY pyproject.toml alembic.ini entrypoint.sh ./
COPY app ./app
COPY alembic ./alembic
RUN pip install --upgrade pip \
    && pip install . \
    && chmod +x entrypoint.sh

# Run as non-root.
RUN useradd --create-home --uid 10001 appuser
USER appuser

EXPOSE 8091

HEALTHCHECK --interval=10s --timeout=3s --start-period=25s --retries=5 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8091/internal/healthz').status==200 else 1)"

ENTRYPOINT ["./entrypoint.sh"]
