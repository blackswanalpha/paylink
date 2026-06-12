"""Golden-vector tests locking the Python signer to the Go chain's crypto (P-256, legacy-Keccak
address, raw r||s signature). Vectors were generated from paylink-chain/internal/crypto."""

from __future__ import annotations

import base64

import pytest
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils

from app.chain import tx_builder
from app.chain.signer import ServiceKeySigner, UnsignedSigner
from app.chain.wire import sha256_hex

# ── golden vectors captured from the Go implementation ──
D_HEX = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
ADDRESS = "0xdee68a30ae64029179d8c146626c3790b40b32a2"
SIGNABLE = (
    '{"type":1,"from":"0xdee68a30ae64029179d8c146626c3790b40b32a2","nonce":0,'
    '"payload":{"paylinkId":"0x1111111111111111111111111111111111111111111111111111111111111111",'
    '"receiver":"0x0000000000000000000000000000000000000004","amount":1500,'
    '"expiry":1893456000,'
    '"metadataHash":"0x2222222222222222222222222222222222222222222222222222222222222222"}}'
)
HASH = "0x1cb94e3a4e7057616748105664feaebcb10e039ccd66adb2560613f966d07f4d"
GO_SIG_B64 = (
    "4AGrH6cLdLAaRRe+bOQlMsjBLsBhE+7hU4VBFz+yywoDmkFuzsSxo+y1iFzn7QyV8zJOyPNDwFeK9Zd298C2uQ=="
)

PLID = "0x1111111111111111111111111111111111111111111111111111111111111111"
RECEIVER = "0x0000000000000000000000000000000000000004"
MDH = "0x2222222222222222222222222222222222222222222222222222222222222222"


def _core() -> dict:
    return tx_builder.build_create(
        pl_id=PLID,
        from_addr=ADDRESS,
        nonce=0,
        receiver=RECEIVER,
        amount=1500,
        expiry=1893456000,
        md_hash=MDH,
    )


def _public_key() -> ec.EllipticCurvePublicKey:
    return ec.derive_private_key(int(D_HEX, 16), ec.SECP256R1()).public_key()


def _verify_raw(sig64: bytes, digest: bytes) -> None:
    r = int.from_bytes(sig64[:32], "big")
    s = int.from_bytes(sig64[32:], "big")
    der = utils.encode_dss_signature(r, s)
    _public_key().verify(der, digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))  # raises if bad


def test_address_matches_go() -> None:
    assert ServiceKeySigner.from_hex(D_HEX).address == ADDRESS


def test_signable_bytes_match_go() -> None:
    assert tx_builder.signable_bytes(_core()).decode() == SIGNABLE


def test_tx_hash_matches_go() -> None:
    assert sha256_hex(tx_builder.signable_bytes(_core())) == HASH


def test_python_verifies_go_signature() -> None:
    # Proves curve (P-256), digest (SHA-256 prehashed) and wire format (raw r||s, base64) all match.
    _verify_raw(base64.standard_b64decode(GO_SIG_B64), bytes.fromhex(HASH[2:]))


def test_sign_digest_roundtrips_and_is_64_bytes() -> None:
    digest = bytes.fromhex(HASH[2:])
    sig64 = base64.standard_b64decode(ServiceKeySigner.from_hex(D_HEX).sign_digest(digest))
    assert len(sig64) == 64
    _verify_raw(sig64, digest)


def test_sign_tx_assembles_wire_fields() -> None:
    tx = tx_builder.sign_tx(_core(), ServiceKeySigner.from_hex(D_HEX))
    assert tx["hash"] == HASH
    assert tx["type"] == 1
    assert tx["from"] == ADDRESS
    assert tx["nonce"] == 0
    assert isinstance(tx["signature"], str) and tx["signature"]


def test_unsigned_signer_has_address_but_empty_signature() -> None:
    s = UnsignedSigner.from_hex(D_HEX)
    assert s.address == ADDRESS
    assert s.sign_digest(bytes.fromhex(HASH[2:])) == ""


def test_build_signer_rejects_unsigned_with_chain_submit_enabled() -> None:
    # The chain verifies every tx signature (ADR-015): an unsigned signer + live chain submission
    # would silently fail every settlement, so the combination must refuse to boot.
    from app.chain.signer import build_signer
    from app.config import Settings

    settings = Settings(signer_mode="unsigned", chain_submit_enabled=True)
    with pytest.raises(ValueError, match="ADR-015"):
        build_signer(settings)

    # Booting without a key stays possible when the chain flow is off (the documented use).
    settings = Settings(signer_mode="unsigned", chain_submit_enabled=False)
    assert build_signer(settings).public_key_b64 == ""
