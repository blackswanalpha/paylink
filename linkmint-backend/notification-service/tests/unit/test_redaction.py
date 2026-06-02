"""PII redaction for logs + responses."""

from __future__ import annotations

from app.redaction import mask_recipient, safe_data_keys


def test_mask_email() -> None:
    assert mask_recipient("email", "jane.doe@example.com") == "j***@example.com"


def test_mask_email_detected_by_at_sign_even_for_other_channel() -> None:
    assert mask_recipient("sms", "a@b.io") == "a***@b.io"


def test_mask_phone_keeps_prefix_and_suffix() -> None:
    masked = mask_recipient("sms", "+254712345678")
    assert masked.startswith("+254")
    assert masked.endswith("78")
    assert "*" in masked
    assert "712345" not in masked


def test_mask_short_value_fully_masked() -> None:
    assert mask_recipient("sms", "1234") == "****"


def test_mask_empty() -> None:
    assert mask_recipient("sms", "") == ""


def test_safe_data_keys_drops_nested() -> None:
    data = {
        "amount": "1500",
        "currency": "KES",
        "n": 3,
        "ok": True,
        "blob": {"pii": "secret"},
        "list": [1, 2],
    }
    assert safe_data_keys(data) == {"amount": "1500", "currency": "KES", "n": 3, "ok": True}


def test_safe_data_keys_empty() -> None:
    assert safe_data_keys(None) == {}
