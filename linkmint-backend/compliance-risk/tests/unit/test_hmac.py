from __future__ import annotations

from app.security.hmac import compute_signature, verify_signature

SECRET = "devnet-callback-secret"
BODY = b'{"user_id":"u1","status":"passed"}'


def test_compute_then_verify_roundtrip() -> None:
    sig = compute_signature(SECRET, BODY)
    assert verify_signature(SECRET, BODY, sig)


def test_sha256_prefix_tolerated() -> None:
    sig = compute_signature(SECRET, BODY)
    assert verify_signature(SECRET, BODY, f"sha256={sig}")
    assert verify_signature(SECRET, BODY, f"SHA256={sig.upper()}")


def test_wrong_secret_fails() -> None:
    sig = compute_signature("other", BODY)
    assert not verify_signature(SECRET, BODY, sig)


def test_tampered_body_fails() -> None:
    sig = compute_signature(SECRET, BODY)
    assert not verify_signature(SECRET, b'{"user_id":"u1","status":"failed"}', sig)


def test_empty_secret_or_presented_is_false() -> None:
    assert not verify_signature("", BODY, compute_signature(SECRET, BODY))
    assert not verify_signature(SECRET, BODY, None)
    assert not verify_signature(SECRET, BODY, "")


def test_garbage_signature_is_false() -> None:
    assert not verify_signature(SECRET, BODY, "not-a-hex-signature")
