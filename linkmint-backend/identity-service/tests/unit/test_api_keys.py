from __future__ import annotations

from app.security.api_keys import PREFIX, generate_api_key, prefix_of, verify_api_key
from app.security.passwords import PasswordHashing
from tests._support import make_settings


def _hasher() -> PasswordHashing:
    return PasswordHashing.from_settings(make_settings())


def test_generate_format_and_verify() -> None:
    h = _hasher()
    gen = generate_api_key(h)
    assert gen.full_key.startswith(PREFIX)
    assert gen.prefix.startswith(PREFIX)
    assert gen.hash not in (gen.full_key, gen.prefix)
    assert verify_api_key(h, gen.full_key, gen.hash)
    assert not verify_api_key(h, "lm_live_wrongkey", gen.hash)


def test_prefix_of_recovers_stored_prefix() -> None:
    h = _hasher()
    gen = generate_api_key(h)
    assert prefix_of(gen.full_key) == gen.prefix
    assert prefix_of("garbage").startswith(PREFIX)
