from __future__ import annotations

from app.security.passwords import PasswordHashing
from tests._support import make_settings


def _hasher() -> PasswordHashing:
    return PasswordHashing.from_settings(make_settings())


def test_hash_verify_roundtrip() -> None:
    h = _hasher()
    hashed = h.hash("s3cret-password")
    assert hashed != "s3cret-password"
    assert h.verify(hashed, "s3cret-password")
    assert not h.verify(hashed, "wrong-password")


def test_verify_rejects_garbage_hash() -> None:
    assert not _hasher().verify("not-a-real-hash", "x")


def test_needs_rehash_false_for_current_params() -> None:
    h = _hasher()
    assert h.needs_rehash(h.hash("x")) is False
