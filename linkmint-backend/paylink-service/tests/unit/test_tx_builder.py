from __future__ import annotations

from app.chain import tx_builder
from app.chain.wire import ZERO_HASH, go_json


def test_new_pl_id_deterministic_with_seed() -> None:
    a = tx_builder.new_pl_id("0xabc", "0xdef", 100, 123, seed="idem-1")
    b = tx_builder.new_pl_id("0xabc", "0xdef", 100, 123, seed="idem-1")
    assert a == b
    assert a.startswith("0x")
    assert len(a) == 66  # 0x + 64 hex


def test_new_pl_id_unique_without_seed() -> None:
    assert tx_builder.new_pl_id("0xa", "0xb", 1, 2) != tx_builder.new_pl_id("0xa", "0xb", 1, 2)


def test_metadata_hash_none_and_empty_are_zero() -> None:
    assert tx_builder.metadata_hash(None) == ZERO_HASH
    assert tx_builder.metadata_hash({}) == ZERO_HASH


def test_metadata_hash_stable_and_nonzero() -> None:
    h1 = tx_builder.metadata_hash({"a": 1, "b": 2})
    h2 = tx_builder.metadata_hash({"a": 1, "b": 2})
    assert h1 == h2 != ZERO_HASH


def test_build_create_field_order_and_omit_rules() -> None:
    core = tx_builder.build_create(
        pl_id="0x1", from_addr="0x2", nonce=3, receiver="0x4", amount=5, expiry=6, md_hash="0x7"
    )
    assert list(core.keys()) == ["type", "from", "nonce", "payload"]
    assert list(core["payload"].keys()) == [
        "paylinkId",
        "receiver",
        "amount",
        "expiry",
        "metadataHash",
    ]
    assert core["type"] == tx_builder.TX_CREATE_PAYLINK
    assert "rules" not in core["payload"]


def test_build_create_includes_rules() -> None:
    core = tx_builder.build_create(
        pl_id="0x1",
        from_addr="0x2",
        nonce=3,
        receiver="0x4",
        amount=5,
        expiry=6,
        md_hash="0x7",
        rules=[{"type": "TimeLock"}],
    )
    assert core["payload"]["rules"] == [{"type": "TimeLock"}]


def test_build_cancel() -> None:
    core = tx_builder.build_cancel(pl_id="0xabc", from_addr="0x2", nonce=9)
    assert core["type"] == tx_builder.TX_CANCEL_PAYLINK
    assert core["payload"] == {"paylinkId": "0xabc"}


def test_go_json_compact_and_html_escaped() -> None:
    assert go_json({"a": 1, "b": "x"}) == b'{"a":1,"b":"x"}'
    assert go_json({"k": "<&>"}) == b'{"k":"\\u003c\\u0026\\u003e"}'
