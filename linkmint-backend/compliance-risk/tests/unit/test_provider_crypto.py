from __future__ import annotations

import base64

import pytest

from app.config import Settings
from app.security.provider_crypto import ProviderCipher, _decode_key
from tests._support import TEST_PROVIDER_KEY, make_settings


def test_encrypt_decrypt_roundtrip() -> None:
    cipher = ProviderCipher.from_settings(make_settings())
    token = cipher.encrypt("session-abc-123")
    assert token != "session-abc-123"
    assert cipher.decrypt(token) == "session-abc-123"


def test_ciphertext_is_nondeterministic() -> None:
    cipher = ProviderCipher.from_settings(make_settings())
    assert cipher.encrypt("x") != cipher.encrypt("x")  # random nonce per encrypt


def test_ephemeral_key_when_unset() -> None:
    # No COMPLIANCE_PROVIDER_ENCRYPTION_KEY → an ephemeral key is generated; still round-trips.
    settings = Settings(provider_encryption_key=None, callback_secrets="stub:s")
    cipher = ProviderCipher.from_settings(settings)
    assert cipher.decrypt(cipher.encrypt("y")) == "y"


def test_decode_key_accepts_hex_base64_and_raw() -> None:
    raw32 = bytes(range(32))
    assert _decode_key(raw32.hex()) == raw32
    assert _decode_key(base64.b64encode(raw32).decode()) == raw32
    assert _decode_key("0" * 32) == b"0" * 32  # 32 literal chars


def test_decode_key_rejects_bad_length() -> None:
    with pytest.raises(ValueError):
        _decode_key("tooshort")


def test_from_settings_with_fixed_key() -> None:
    cipher = ProviderCipher.from_settings(make_settings(provider_encryption_key=TEST_PROVIDER_KEY))
    assert cipher.decrypt(cipher.encrypt("ref")) == "ref"


def test_bad_version_byte_rejected() -> None:
    cipher = ProviderCipher.from_settings(make_settings())
    token = cipher.encrypt("z")
    raw = bytearray(base64.b64decode(token))
    raw[0] = 0x99  # corrupt the version byte
    with pytest.raises(ValueError):
        cipher.decrypt(base64.b64encode(bytes(raw)).decode())


def test_constructor_rejects_wrong_key_length() -> None:
    with pytest.raises(ValueError):
        ProviderCipher(b"short")
