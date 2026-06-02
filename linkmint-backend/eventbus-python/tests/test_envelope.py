from __future__ import annotations

from pathlib import Path

from linkmint_eventbus.envelope import Envelope, _canonical

LOCAL_GOLDEN = Path(__file__).parent.parent / "testdata" / "envelope.golden.json"
# The Go golden lives in the sibling eventbus-go module; present when the full repo is checked out.
GO_GOLDEN = Path(__file__).parents[2] / "eventbus-go" / "testdata" / "envelope.golden.json"


def _golden_envelope() -> Envelope:
    # payload keys are deliberately NOT alphabetical (pl_id, amount, currency) to prove sorting.
    return Envelope(
        id="00000000-0000-0000-0000-000000000001",
        name="paylink.verified",
        key="PLK_demo",
        correlation_id="trace-abc",
        occurred_at="2026-06-01T12:00:00Z",
        source="paylink-service",
        payload={"pl_id": "PLK_demo", "amount": "1000", "currency": "KES"},
    )


def test_to_canonical_bytes_matches_golden() -> None:
    want = LOCAL_GOLDEN.read_bytes().rstrip(b"\n")
    assert _golden_envelope().to_canonical_bytes() == want


def test_golden_files_byte_identical_to_go() -> None:
    # The Python and Go goldens must be byte-identical — the cross-language wire-compat drift guard.
    assert GO_GOLDEN.exists(), f"Go golden missing at {GO_GOLDEN}"
    assert LOCAL_GOLDEN.read_bytes().rstrip(b"\n") == GO_GOLDEN.read_bytes().rstrip(b"\n")


def test_canonical_sorts_keys_recursively() -> None:
    assert _canonical({"z": "1", "a": {"y": "2", "x": "3"}}) == {
        "a": {"x": "3", "y": "2"},
        "z": "1",
    }


def test_canonical_bytes_does_not_html_escape() -> None:
    env = Envelope(
        id="x",
        name="n",
        occurred_at="2026-06-01T12:00:00Z",
        source="s",
        payload={"note": "a<b&c>d"},
    )
    assert b'{"note":"a<b&c>d"}' in env.to_canonical_bytes()


def test_round_trip() -> None:
    raw = _golden_envelope().to_canonical_bytes()
    got = Envelope.from_bytes(raw)
    assert got.name == "paylink.verified"
    assert got.payload == {"amount": "1000", "currency": "KES", "pl_id": "PLK_demo"}


def test_from_bytes_ignores_unknown_fields() -> None:
    raw = (
        b'{"id":"x","name":"paylink.verified","extra":"ignored",'
        b'"occurred_at":"2026-06-01T12:00:00Z","source":"s","payload":{"a":1}}'
    )
    got = Envelope.from_bytes(raw)
    assert got.name == "paylink.verified"
    assert got.payload == {"a": 1}


def test_new_stamps_id_and_timestamp() -> None:
    e = Envelope.new("payment.failed", "PMT_1", "trace", "payment-orchestrator", {"reason": "x"})
    assert e.id
    assert e.occurred_at.endswith("Z")
    assert len(e.occurred_at) == len("2026-06-01T12:00:00Z")
    assert e.payload == {"reason": "x"}


def test_new_with_none_payload_is_empty_object() -> None:
    e = Envelope.new("paylink.created", "PLK_1", "", "paylink-service", None)
    assert e.payload == {}
    assert b'"payload":{}' in e.to_canonical_bytes()
