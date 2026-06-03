"""Build the configured FX provider (static by default; http seam behind a flag)."""

from __future__ import annotations

import httpx

from app.config import Settings
from app.fx.provider import FxProvider
from app.fx.static import StaticFxProvider


def build_fx_provider(
    settings: Settings, http_client: httpx.AsyncClient | None = None
) -> FxProvider:
    if settings.fx_provider == "http" and settings.fx_http_url:
        from app.fx.http import HttpFxProvider

        if http_client is None:  # pragma: no cover - lifespan always supplies the client
            http_client = httpx.AsyncClient()
        return HttpFxProvider(
            http_client, settings.fx_http_url, timeout=settings.fx_http_timeout_seconds
        )
    return StaticFxProvider(settings.fx_static_rates)
