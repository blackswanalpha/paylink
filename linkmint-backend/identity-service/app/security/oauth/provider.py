"""OAuth provider seam.

A provider turns an authorization ``code`` into a normalized :class:`OAuthIdentity`
``{provider, subject, email}`` — exactly the fields the ``identity.oauth_identities`` table keys on.
The concrete provider (fake vs real Google/Apple/GitHub) is selected by config in
:mod:`app.security.oauth.registry`, so the auth service stays provider-agnostic.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass


@dataclass(frozen=True)
class OAuthIdentity:
    provider: str
    subject: str
    email: str | None


@dataclass(frozen=True)
class AuthorizeRequest:
    authorize_url: str
    state: str


class OAuthError(Exception):
    """Raised by a provider when authorize/exchange fails (translated to OAUTH_EXCHANGE_FAILED)."""


class OAuthProvider(ABC):
    @abstractmethod
    def authorize(self, *, state: str, redirect_uri: str | None = None) -> AuthorizeRequest: ...

    @abstractmethod
    async def exchange_code(
        self, *, code: str, state: str, redirect_uri: str | None = None
    ) -> OAuthIdentity: ...
