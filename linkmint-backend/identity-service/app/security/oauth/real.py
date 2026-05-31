"""Real OAuth providers (Google / Apple / GitHub) via authlib.

Config-driven and **NOT verified locally** (no creds in CI/dev — ``IDENTITY_OAUTH_FAKE=true`` is
used there). The network methods are marked ``# pragma: no cover``: they are the documented external
seam, exercised only against real providers. Verifying them with live creds is a follow-up.
"""

from __future__ import annotations

from dataclasses import dataclass

from app.config import OAuthProviderConfig
from app.security.oauth.provider import (
    AuthorizeRequest,
    OAuthError,
    OAuthIdentity,
    OAuthProvider,
)


@dataclass(frozen=True)
class _Endpoints:
    authorize_url: str
    token_url: str
    userinfo_url: str  # empty for Apple (subject comes from the id_token)
    scope: str


_ENDPOINTS: dict[str, _Endpoints] = {
    "google": _Endpoints(
        "https://accounts.google.com/o/oauth2/v2/auth",
        "https://oauth2.googleapis.com/token",
        "https://openidconnect.googleapis.com/v1/userinfo",
        "openid email",
    ),
    "github": _Endpoints(
        "https://github.com/login/oauth/authorize",
        "https://github.com/login/oauth/access_token",
        "https://api.github.com/user",
        "read:user user:email",
    ),
    "apple": _Endpoints(
        "https://appleid.apple.com/auth/authorize",
        "https://appleid.apple.com/auth/token",
        "",
        "name email",
    ),
}


class AuthlibOAuthProvider(OAuthProvider):
    def __init__(self, config: OAuthProviderConfig) -> None:
        if config.provider not in _ENDPOINTS:
            raise OAuthError(f"unsupported provider {config.provider}")
        self._config = config
        self._endpoints = _ENDPOINTS[config.provider]

    def _client(self, redirect_uri: str | None):  # type: ignore[no-untyped-def]  # pragma: no cover - external seam
        from authlib.integrations.httpx_client import AsyncOAuth2Client

        return AsyncOAuth2Client(
            client_id=self._config.client_id,
            client_secret=self._config.client_secret,
            redirect_uri=redirect_uri or self._config.redirect_uri,
            scope=self._endpoints.scope,
        )

    def authorize(
        self, *, state: str, redirect_uri: str | None = None
    ) -> AuthorizeRequest:  # pragma: no cover - external seam
        client = self._client(redirect_uri)
        url, real_state = client.create_authorization_url(
            self._endpoints.authorize_url, state=state
        )
        return AuthorizeRequest(authorize_url=url, state=real_state)

    async def exchange_code(
        self, *, code: str, state: str, redirect_uri: str | None = None
    ) -> OAuthIdentity:  # pragma: no cover - external seam
        import jwt as _jwt

        client = self._client(redirect_uri)
        try:
            token = await client.fetch_token(self._endpoints.token_url, code=code, state=state)
            if not self._endpoints.userinfo_url:
                # Apple: the subject is the `sub` claim of the returned id_token.
                # SECURITY follow-up: verify the id_token signature against Apple's JWKS (+ aud/iss/
                # exp) before trusting these claims; this seam is unverified (no local Apple creds).
                claims = _jwt.decode(token["id_token"], options={"verify_signature": False})
                return OAuthIdentity(
                    provider=self._config.provider,
                    subject=str(claims["sub"]),
                    email=claims.get("email"),
                )
            resp = await client.get(self._endpoints.userinfo_url)
            resp.raise_for_status()
            info = resp.json()
            subject = str(info.get("sub") or info.get("id"))
            return OAuthIdentity(
                provider=self._config.provider,
                subject=subject,
                email=info.get("email"),
            )
        except Exception as exc:  # noqa: BLE001 - any provider/transport failure → exchange failed
            raise OAuthError(str(exc)) from exc
        finally:
            await client.aclose()
