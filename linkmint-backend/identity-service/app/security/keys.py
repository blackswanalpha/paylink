"""RS256 key management: load-or-generate the signing keypair, expose the JWKS.

The private key is NEVER in code — it comes from ``IDENTITY_JWT_PRIVATE_KEY_PEM`` (env/KMS). When
unset, an ephemeral RSA-2048 keypair is generated at startup so local dev is zero-config (mirrors
the paylink-service signer seam). The public key feeds the ``/.well-known/jwks.json`` endpoint and
the gateway's additive RS256 consumer.
"""

from __future__ import annotations

import base64
import hashlib
from typing import Any

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.asymmetric.rsa import RSAPrivateKey

from app.config import Settings


def _b64url_uint(value: int) -> str:
    """Base64url-encode a big-endian unsigned integer (JWK ``n``/``e`` form, no padding)."""
    length = (value.bit_length() + 7) // 8 or 1
    return base64.urlsafe_b64encode(value.to_bytes(length, "big")).rstrip(b"=").decode()


class KeyStore:
    """Holds the RS256 keypair + derived ``kid`` and JWKS."""

    def __init__(self, private_key: RSAPrivateKey) -> None:
        self._private_key = private_key
        self._public_key = private_key.public_key()
        self._private_pem = private_key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
        self._public_pem = self._public_key.public_bytes(
            serialization.Encoding.PEM,
            serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        der = self._public_key.public_bytes(
            serialization.Encoding.DER,
            serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        # Deterministic key id: base64url(SHA-256(DER SubjectPublicKeyInfo)).
        self._kid = base64.urlsafe_b64encode(hashlib.sha256(der).digest()).rstrip(b"=").decode()

    @classmethod
    def from_settings(cls, settings: Settings) -> KeyStore:
        if settings.jwt_private_key_pem is not None:
            pem = settings.jwt_private_key_pem.get_secret_value().encode()
            key = serialization.load_pem_private_key(pem, password=None)
            if not isinstance(key, RSAPrivateKey):
                raise ValueError("IDENTITY_JWT_PRIVATE_KEY_PEM must be an RSA private key")
            return cls(key)
        # Ephemeral dev key (prod injects the PEM via env/KMS).
        return cls(rsa.generate_private_key(public_exponent=65537, key_size=2048))

    @property
    def kid(self) -> str:
        return self._kid

    @property
    def private_pem(self) -> bytes:
        return self._private_pem

    @property
    def public_pem(self) -> str:
        return self._public_pem.decode()

    def jwks(self) -> dict[str, Any]:
        numbers = self._public_key.public_numbers()
        return {
            "keys": [
                {
                    "kty": "RSA",
                    "use": "sig",
                    "alg": "RS256",
                    "kid": self._kid,
                    "n": _b64url_uint(numbers.n),
                    "e": _b64url_uint(numbers.e),
                }
            ]
        }
