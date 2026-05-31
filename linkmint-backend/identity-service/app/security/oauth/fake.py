"""Deterministic fake OAuth provider.

Selected by ``IDENTITY_OAUTH_FAKE=true`` (the work04 ``DARAJA_STUB`` analog): no network, no creds,
fully unit-testable. ``exchange_code`` derives a stable ``subject``/``email`` from the code so tests
can assert that the same code links to the same user.
"""

from __future__ import annotations

import hashlib

from app.security.oauth.provider import (
    AuthorizeRequest,
    OAuthError,
    OAuthIdentity,
    OAuthProvider,
)


class FakeOAuthProvider(OAuthProvider):
    def __init__(self, provider: str) -> None:
        self._provider = provider

    def authorize(self, *, state: str, redirect_uri: str | None = None) -> AuthorizeRequest:
        url = f"https://fake-oauth.local/{self._provider}/authorize?state={state}"
        return AuthorizeRequest(authorize_url=url, state=state)

    async def exchange_code(
        self, *, code: str, state: str, redirect_uri: str | None = None
    ) -> OAuthIdentity:
        if not code:
            raise OAuthError("missing authorization code")
        subject = hashlib.sha256(f"{self._provider}:{code}".encode()).hexdigest()[:32]
        email = f"{subject}@fake-{self._provider}.local"
        return OAuthIdentity(provider=self._provider, subject=subject, email=email)
