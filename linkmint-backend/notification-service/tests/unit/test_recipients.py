"""Recipient resolvers — inline (Phase-1) + identity (deferred seam, respx-mocked)."""

from __future__ import annotations

import uuid

import httpx
import respx

from app.domain.models import Channel
from app.recipients.identity import IdentityRecipientResolver
from app.recipients.inline import InlineRecipientResolver

IDENTITY_BASE = "https://identity.test"


async def test_inline_resolves_from_contact() -> None:
    uid = uuid.uuid4()
    r = await InlineRecipientResolver().resolve(
        uid, {"phone": "+254712345678", "email": "a@b.io", "locale": "sw"}
    )
    assert r.user_id == uid
    assert r.address_for(Channel.SMS) == "+254712345678"
    assert r.address_for(Channel.EMAIL) == "a@b.io"
    assert r.locale == "sw"


async def test_inline_missing_contact_yields_no_addresses() -> None:
    r = await InlineRecipientResolver().resolve(uuid.uuid4(), None)
    assert r.address_for(Channel.SMS) is None
    assert r.address_for(Channel.EMAIL) is None


async def test_identity_resolver_fetches_contact() -> None:
    uid = uuid.uuid4()
    with respx.mock:
        respx.get(f"{IDENTITY_BASE}/internal/contacts/{uid}").mock(
            return_value=httpx.Response(200, json={"phone": "+254700000000", "email": "x@y.io"})
        )
        async with httpx.AsyncClient() as client:
            resolver = IdentityRecipientResolver(client, base_url=IDENTITY_BASE, internal_token="t")
            r = await resolver.resolve(uid, None)
    assert r.phone == "+254700000000"
    assert r.email == "x@y.io"


async def test_identity_resolver_miss_returns_contactless() -> None:
    uid = uuid.uuid4()
    with respx.mock:
        respx.get(f"{IDENTITY_BASE}/internal/contacts/{uid}").mock(return_value=httpx.Response(404))
        async with httpx.AsyncClient() as client:
            resolver = IdentityRecipientResolver(client, base_url=IDENTITY_BASE)
            r = await resolver.resolve(uid, None)
    assert r.phone is None and r.email is None


async def test_identity_resolver_network_error_returns_contactless() -> None:
    uid = uuid.uuid4()
    with respx.mock:
        respx.get(f"{IDENTITY_BASE}/internal/contacts/{uid}").mock(
            side_effect=httpx.ConnectError("down")
        )
        async with httpx.AsyncClient() as client:
            resolver = IdentityRecipientResolver(client, base_url=IDENTITY_BASE)
            r = await resolver.resolve(uid, None)
    assert r.phone is None and r.email is None
