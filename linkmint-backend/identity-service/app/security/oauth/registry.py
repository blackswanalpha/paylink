"""Build the OAuth provider resolver from settings (fake vs real), mirroring build_publisher."""

from __future__ import annotations

from app.config import Settings
from app.security.oauth.fake import FakeOAuthProvider
from app.security.oauth.provider import OAuthProvider
from app.security.oauth.real import AuthlibOAuthProvider

_SUPPORTED = ("google", "apple", "github")


class OAuthResolver:
    def __init__(self, providers: dict[str, OAuthProvider]) -> None:
        self._providers = providers

    def get(self, provider: str) -> OAuthProvider | None:
        return self._providers.get(provider)


def build_oauth_resolver(settings: Settings) -> OAuthResolver:
    providers: dict[str, OAuthProvider] = {}
    if settings.oauth_fake:
        for name in _SUPPORTED:
            providers[name] = FakeOAuthProvider(name)
        return OAuthResolver(providers)
    for name in _SUPPORTED:
        cfg = settings.oauth_provider_config(name)
        if cfg is not None:
            providers[name] = AuthlibOAuthProvider(cfg)
    return OAuthResolver(providers)
