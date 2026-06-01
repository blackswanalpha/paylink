from __future__ import annotations

from app.redaction import redact


def test_keeps_allowlisted_scalars() -> None:
    out = redact(
        {
            "session_id": "s-123",
            "status": "passed",
            "reason_code": "ok",
            "document_type": "passport",
            "verification_level": "L2",
            "country": "KE",
            "tier": 2,
        }
    )
    assert out == {
        "session_id": "s-123",
        "status": "passed",
        "reason_code": "ok",
        "document_type": "passport",
        "verification_level": "L2",
        "country": "KE",
        "tier": 2,
    }


def test_drops_pii_keys() -> None:
    out = redact(
        {
            "status": "passed",
            "full_name": "Jane Doe",
            "national_id": "12345678",
            "date_of_birth": "1990-01-01",
            "address": "1 Main St",
            "document_image": "base64...",
            "selfie": "base64...",
        }
    )
    assert out == {"status": "passed"}
    assert "full_name" not in out
    assert "national_id" not in out


def test_drops_nested_structures_even_under_allowlisted_key() -> None:
    # A non-scalar value under an allowlisted key is dropped (can't smuggle a PII blob through).
    out = redact({"status": {"nested": "pii"}, "session_id": ["a", "b"], "country": "KE"})
    assert out == {"country": "KE"}


def test_none_and_empty() -> None:
    assert redact(None) == {}
    assert redact({}) == {}


def test_scalar_bool_and_none_pass() -> None:
    out = redact({"status": None, "tier": 0, "completed_at": "2026-01-01T00:00:00Z"})
    assert out == {"status": None, "tier": 0, "completed_at": "2026-01-01T00:00:00Z"}
