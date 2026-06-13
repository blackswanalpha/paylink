"""HMAC webhook-signature verification."""

from __future__ import annotations

from app.security.hmac import compute_signature, verify_signature

SECRET = "s3cr3t"
RAW = b'{"kind":"dispute.opened"}'


def test_valid_signature() -> None:
    sig = compute_signature(SECRET, RAW)
    assert verify_signature(SECRET, RAW, sig) is True


def test_sha256_prefix_tolerated() -> None:
    sig = compute_signature(SECRET, RAW)
    assert verify_signature(SECRET, RAW, f"sha256={sig}") is True


def test_missing_signature_rejected() -> None:
    assert verify_signature(SECRET, RAW, None) is False
    assert verify_signature(SECRET, RAW, "") is False


def test_empty_secret_rejected() -> None:
    assert verify_signature("", RAW, compute_signature(SECRET, RAW)) is False


def test_wrong_signature_rejected() -> None:
    assert verify_signature(SECRET, RAW, "deadbeef") is False


def test_tampered_body_rejected() -> None:
    sig = compute_signature(SECRET, RAW)
    assert verify_signature(SECRET, b'{"kind":"dispute.resolved"}', sig) is False
