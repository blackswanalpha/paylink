from __future__ import annotations

import base64

import pytest
from cryptography.exceptions import InvalidTag

from app.security.bank_crypto import BankCipher, _decode_key
from tests._support import TEST_BANK_KEY, make_settings


def test_encrypt_decrypt_roundtrip() -> None:
    cipher = BankCipher.from_settings(make_settings())
    ct = cipher.encrypt("254700123456")
    assert ct != "254700123456"
    assert cipher.decrypt(ct) == "254700123456"


def test_ciphertext_does_not_leak_plaintext() -> None:
    cipher = BankCipher.from_settings(make_settings())
    ct = cipher.encrypt("ACME-IBAN-GB29NWBK60161331926819")
    # The raw account number must not appear in the stored ciphertext token.
    assert "GB29NWBK60161331926819" not in ct


def test_distinct_nonce_per_encrypt() -> None:
    cipher = BankCipher.from_settings(make_settings())
    assert cipher.encrypt("x") != cipher.encrypt("x")


def test_tamper_detected() -> None:
    cipher = BankCipher.from_settings(make_settings())
    raw = bytearray(base64.b64decode(cipher.encrypt("x")))
    raw[-1] ^= 0x01
    with pytest.raises(InvalidTag):
        cipher.decrypt(base64.b64encode(bytes(raw)).decode())


def test_unsupported_version_rejected() -> None:
    cipher = BankCipher.from_settings(make_settings())
    raw = bytearray(base64.b64decode(cipher.encrypt("x")))
    raw[0] = 0x02  # bogus version byte
    with pytest.raises(ValueError, match="version"):
        cipher.decrypt(base64.b64encode(bytes(raw)).decode())


def test_ephemeral_key_when_unset() -> None:
    # No key configured → an ephemeral 256-bit key is generated; roundtrip still works.
    cipher = BankCipher.from_settings(make_settings(bank_encryption_key=None))
    assert cipher.decrypt(cipher.encrypt("hello")) == "hello"


def test_decode_key_accepts_forms() -> None:
    assert len(_decode_key(TEST_BANK_KEY)) == 32  # base64
    assert len(_decode_key("ab" * 32)) == 32  # hex
    assert len(_decode_key("z" * 32)) == 32  # raw utf-8
    with pytest.raises(ValueError):
        _decode_key("too-short")
