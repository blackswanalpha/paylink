from __future__ import annotations

import pytest

from app.security.oauth.fake import FakeOAuthProvider
from app.security.oauth.provider import OAuthError
from app.security.oauth.registry import build_oauth_resolver
from tests._support import make_settings


def test_authorize_url_carries_state() -> None:
    authz = FakeOAuthProvider("google").authorize(state="st-123")
    assert "state=st-123" in authz.authorize_url
    assert authz.state == "st-123"


async def test_exchange_is_deterministic() -> None:
    prov = FakeOAuthProvider("google")
    a = await prov.exchange_code(code="abc", state="s")
    b = await prov.exchange_code(code="abc", state="s")
    assert a.subject == b.subject
    assert a.provider == "google"
    assert a.email is not None and a.email.endswith("@fake-google.local")


async def test_exchange_rejects_empty_code() -> None:
    with pytest.raises(OAuthError):
        await FakeOAuthProvider("google").exchange_code(code="", state="s")


def test_resolver_fake_mode() -> None:
    resolver = build_oauth_resolver(make_settings(oauth_fake=True))
    assert resolver.get("google") is not None
    assert resolver.get("apple") is not None
    assert resolver.get("twitter") is None


def test_resolver_real_mode_without_creds_is_empty() -> None:
    resolver = build_oauth_resolver(make_settings(oauth_fake=False))
    assert resolver.get("google") is None
