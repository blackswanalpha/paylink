from __future__ import annotations

import base64

import pytest
from cryptography.exceptions import InvalidTag

from app.security.mfa_crypto import MfaCipher, _decode_key
from tests._support import TEST_MFA_KEY, make_settings


def test_encrypt_decrypt_roundtrip() -> None:
    cipher = MfaCipher.from_settings(make_settings())
    ct = cipher.encrypt("my-totp-secret")
    assert ct != "my-totp-secret"
    assert cipher.decrypt(ct) == "my-totp-secret"


def test_distinct_nonce_per_encrypt() -> None:
    cipher = MfaCipher.from_settings(make_settings())
    assert cipher.encrypt("x") != cipher.encrypt("x")


def test_tamper_detected() -> None:
    cipher = MfaCipher.from_settings(make_settings())
    raw = bytearray(base64.b64decode(cipher.encrypt("x")))
    raw[-1] ^= 0x01
    with pytest.raises(InvalidTag):
        cipher.decrypt(base64.b64encode(bytes(raw)).decode())


def test_decode_key_accepts_forms() -> None:
    assert len(_decode_key(TEST_MFA_KEY)) == 32  # base64
    assert len(_decode_key("ab" * 32)) == 32  # hex
    assert len(_decode_key("z" * 32)) == 32  # raw utf-8
    with pytest.raises(ValueError):
        _decode_key("too-short")
