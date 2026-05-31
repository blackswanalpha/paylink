from __future__ import annotations

import pytest

from app.storage.object_store import LocalObjectStore, build_object_store
from tests._support import FakeObjectStore, make_settings


def test_fake_object_store_roundtrip() -> None:
    store = FakeObjectStore()
    store.put("k/1", b"hello", content_type="text/plain")
    assert store.get("k/1") == b"hello"


def test_local_object_store_roundtrip(tmp_path: object) -> None:
    store = LocalObjectStore(str(tmp_path))
    store.put("merchant/abc/doc1", b"\x00\x01\x02")
    assert store.get("merchant/abc/doc1") == b"\x00\x01\x02"


def test_local_object_store_rejects_traversal(tmp_path: object) -> None:
    store = LocalObjectStore(str(tmp_path))
    with pytest.raises(ValueError, match="escapes"):
        store.put("../../etc/passwd", b"x")


def test_build_object_store_defaults_to_local(tmp_path: object) -> None:
    settings = make_settings(object_store_mode="local", local_object_store_dir=str(tmp_path))
    store = build_object_store(settings)
    assert isinstance(store, LocalObjectStore)
