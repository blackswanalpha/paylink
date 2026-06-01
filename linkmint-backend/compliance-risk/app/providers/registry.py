"""KYC provider registry — the single swap point (stub in dev/tests, HTTP vendor in prod) plus the
per-provider callback HMAC secret lookup.

The ``stub`` provider is ALWAYS registered (the MVP default + the integration smoke path). When
``COMPLIANCE_KYC_PROVIDER=http``, an :class:`~app.providers.http.HttpKycProvider` named ``http`` is
added (and may be reached at ``/v1/kyc/callbacks/http``). Callback secrets come from
``Settings.callback_secrets_map()``; a provider with no configured secret can never have a callback
verified (``verify_signature`` returns False on an empty secret → 401).
"""

from __future__ import annotations

import httpx

from app.config import Settings
from app.providers.base import KycProvider
from app.providers.fake import FakeKycProvider
from app.providers.http import HttpKycProvider


class KycProviderRegistry:
    def __init__(self, providers: list[KycProvider], secrets: dict[str, str]) -> None:
        self._providers: dict[str, KycProvider] = {p.name: p for p in providers}
        self._secrets = secrets

    def default(self) -> KycProvider:
        """The configured default provider (the first registered)."""
        return next(iter(self._providers.values()))

    def get(self, name: str) -> KycProvider | None:
        return self._providers.get(name)

    def secret_for(self, name: str) -> str:
        """The callback HMAC secret for ``name`` (empty if none configured → callbacks 401)."""
        return self._secrets.get(name, "")


def build_registry(settings: Settings, client: httpx.AsyncClient) -> KycProviderRegistry:
    """Wire the providers from config. The stub is always present; http is added when selected."""
    providers: list[KycProvider] = [FakeKycProvider()]
    if settings.kyc_provider == "http" and settings.kyc_provider_url:
        providers.insert(0, HttpKycProvider("http", client, settings.kyc_provider_url))
    return KycProviderRegistry(providers, settings.callback_secrets_map())
