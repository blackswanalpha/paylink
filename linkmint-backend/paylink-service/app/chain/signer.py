"""Transaction signing seam.

The lVM uses **NIST P-256** ECDSA (``paylink-chain/internal/crypto/keys.go``):
  - sign the 32-byte SHA-256 digest of ``SignableBytes``; signature is raw ``r||s`` (64 bytes),
    base64-encoded on the wire (Go marshals ``[]byte`` as base64);
  - the tx carries ``pubKey`` = the uncompressed public key (65 bytes: ``0x04 || X || Y``,
    base64) — P-256 has no key recovery, and the chain checks the key derives ``From``;
  - address = last 20 bytes of **legacy Keccak-256** of the uncompressed pubkey ``X||Y``;
  - the private key is the big-endian ``D`` scalar (same hex format as ``paylinkd --privkey``).

The chain **verifies signatures at admission and in block execution** (supersedes ADR-005):
``UnsignedSigner`` transactions are now REJECTED by the chain — it remains only so a deployment
can boot without a key for non-chain flows. The seam lets a future client-signed flow
(SDK / work05) swap the implementation without touching the service.
"""

from __future__ import annotations

import base64
from typing import Protocol

from Crypto.Hash import keccak
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils

from app.config import Settings


def _legacy_keccak256(data: bytes) -> bytes:
    digest = keccak.new(digest_bits=256)
    digest.update(data)
    return digest.digest()


def address_from_public_key(public_key: ec.EllipticCurvePublicKey) -> str:
    raw = public_key.public_bytes(
        serialization.Encoding.X962,
        serialization.PublicFormat.UncompressedPoint,
    )  # 65 bytes: 0x04 || X || Y
    return "0x" + _legacy_keccak256(raw[1:])[12:].hex()


class Signer(Protocol):
    @property
    def address(self) -> str: ...

    @property
    def public_key_b64(self) -> str: ...

    def sign_digest(self, digest: bytes) -> str: ...


class ServiceKeySigner:
    """Signs with a service-held P-256 key."""

    def __init__(self, private_key: ec.EllipticCurvePrivateKey) -> None:
        self._private_key = private_key
        self._address = address_from_public_key(private_key.public_key())
        self._public_key_b64 = base64.standard_b64encode(
            private_key.public_key().public_bytes(
                serialization.Encoding.X962,
                serialization.PublicFormat.UncompressedPoint,
            )
        ).decode()

    @classmethod
    def from_hex(cls, key_hex: str) -> ServiceKeySigner:
        d = int(key_hex.removeprefix("0x"), 16)
        return cls(ec.derive_private_key(d, ec.SECP256R1()))

    @classmethod
    def generate(cls) -> ServiceKeySigner:
        return cls(ec.generate_private_key(ec.SECP256R1()))

    @property
    def address(self) -> str:
        return self._address

    @property
    def public_key_b64(self) -> str:
        return self._public_key_b64

    def sign_digest(self, digest: bytes) -> str:
        der = self._private_key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
        r, s = utils.decode_dss_signature(der)
        sig = r.to_bytes(32, "big") + s.to_bytes(32, "big")
        return base64.standard_b64encode(sig).decode()


class UnsignedSigner(ServiceKeySigner):
    """Supplies a ``From`` address but no signature/pubkey.

    The chain now REJECTS such transactions — only useful for booting without a key
    when the chain flow is disabled.
    """

    @property
    def public_key_b64(self) -> str:
        return ""

    def sign_digest(self, digest: bytes) -> str:
        return ""


def build_signer(settings: Settings) -> Signer:
    if settings.signer_mode == "unsigned" and settings.chain_submit_enabled:
        raise ValueError(
            "PAYLINK_SIGNER_MODE=unsigned cannot be combined with chain submission: the chain "
            "verifies every tx signature (ADR-015), so every submit would be rejected. Provide a "
            "key (service_key) or set PAYLINK_CHAIN_SUBMIT_ENABLED=false."
        )
    cls: type[ServiceKeySigner] = (
        UnsignedSigner if settings.signer_mode == "unsigned" else ServiceKeySigner
    )
    key = settings.chain_signer_key.get_secret_value() if settings.chain_signer_key else ""
    return cls.from_hex(key) if key else cls.generate()
