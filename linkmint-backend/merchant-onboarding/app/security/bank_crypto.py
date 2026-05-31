"""Bank-account-ref-at-rest encryption — the KMS stand-in.

The spec marks ``bank_accounts.account_ref`` as "KMS-encrypted". There's no KMS locally, so we
envelope the bank account details with AES-256-GCM under ``MERCHANT_BANK_ENCRYPTION_KEY``
(env/KMS-sourced; ephemeral dev key when unset). A leading version byte makes future key rotation
tractable (re-encrypt under a new version) — that rotation job is a documented follow-up.

INVARIANT: the plaintext account details are encrypted to ``account_ref`` immediately and the
plaintext is NEVER persisted, logged, returned in any response, or placed in an event payload.
Modeled exactly on identity-service's ``MfaCipher``.
"""

from __future__ import annotations

import base64
import os

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from app.config import Settings

_VERSION = b"\x01"
_NONCE_LEN = 12


def _decode_key(raw: str) -> bytes:
    """Accept a 32-byte key as hex, base64(url), or a literal 32-char utf-8 string."""
    s = raw.strip()
    try:
        key = bytes.fromhex(s)
        if len(key) == 32:
            return key
    except ValueError:
        pass
    body = s.rstrip("=")
    padded = body + "=" * (-len(body) % 4)
    for decoder in (base64.b64decode, base64.urlsafe_b64decode):
        try:
            key = decoder(padded)
            if len(key) == 32:
                return key
        except (ValueError, TypeError):
            pass
    kb = s.encode()
    if len(kb) == 32:
        return kb
    raise ValueError("MERCHANT_BANK_ENCRYPTION_KEY must be 32 bytes (hex, base64, or raw)")


class BankCipher:
    def __init__(self, key: bytes) -> None:
        if len(key) != 32:
            raise ValueError("bank encryption key must be 32 bytes")
        self._aes = AESGCM(key)

    @classmethod
    def from_settings(cls, settings: Settings) -> BankCipher:
        secret = settings.bank_encryption_key
        raw = secret.get_secret_value() if secret else ""
        key = _decode_key(raw) if raw else AESGCM.generate_key(bit_length=256)
        return cls(key)

    def encrypt(self, plaintext: str) -> str:
        nonce = os.urandom(_NONCE_LEN)
        ct = self._aes.encrypt(nonce, plaintext.encode(), None)
        return base64.b64encode(_VERSION + nonce + ct).decode()

    def decrypt(self, token: str) -> str:
        raw = base64.b64decode(token.encode())
        version, nonce, ct = raw[:1], raw[1 : 1 + _NONCE_LEN], raw[1 + _NONCE_LEN :]
        if version != _VERSION:
            raise ValueError("unsupported bank ciphertext version")
        return self._aes.decrypt(nonce, ct, None).decode()
