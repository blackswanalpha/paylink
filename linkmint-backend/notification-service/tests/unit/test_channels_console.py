"""Console (sandbox) channel providers — deterministic ref, fail-mode, redacted logging."""

from __future__ import annotations

import pytest

from app.channels.base import SendError
from app.channels.console import ConsoleEmailProvider, ConsoleSmsProvider


def test_sms_send_deterministic_ref() -> None:
    p = ConsoleSmsProvider()
    r1 = p.send(to="+254712345678", body="hi")
    r2 = p.send(to="+254712345678", body="hi")
    assert r1.provider == "console"
    assert r1.provider_ref == r2.provider_ref
    assert r1.provider_ref.startswith("console-")


def test_sms_different_input_different_ref() -> None:
    p = ConsoleSmsProvider()
    assert (
        p.send(to="+254700000001", body="a").provider_ref
        != p.send(to="+254700000002", body="a").provider_ref
    )


def test_fail_mode_raises_send_error() -> None:
    p = ConsoleSmsProvider(frozenset({"+254700000000"}))
    with pytest.raises(SendError):
        p.send(to="+254700000000", body="boom")


def test_email_send_ok() -> None:
    r = ConsoleEmailProvider().send(to="jane@example.com", body="hi", subject="Subject")
    assert r.provider == "console"
    assert r.provider_ref
