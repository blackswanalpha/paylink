"""Channel registry — console default, HTTP selection by config, channel lookup."""

from __future__ import annotations

import httpx

from app.channels.console import ConsoleEmailProvider, ConsoleSmsProvider
from app.channels.http_email import SendGridEmailProvider
from app.channels.http_sms import AfricasTalkingSmsProvider
from app.channels.registry import build_channel_registry
from tests._support import make_settings


def test_console_is_the_default() -> None:
    with httpx.Client() as client:
        reg = build_channel_registry(make_settings(), client)
    assert isinstance(reg.sms(), ConsoleSmsProvider)
    assert isinstance(reg.email(), ConsoleEmailProvider)


def test_for_channel_routes_by_name() -> None:
    with httpx.Client() as client:
        reg = build_channel_registry(make_settings(), client)
    assert reg.for_channel("sms") is reg.sms()
    assert reg.for_channel("email") is reg.email()
    assert reg.for_channel("push") is None


def test_http_selected_when_configured() -> None:
    settings = make_settings(
        sms_provider="http",
        email_provider="http",
        africastalking_api_key="at-key",
        sendgrid_api_key="sg-key",
    )
    with httpx.Client() as client:
        reg = build_channel_registry(settings, client)
    assert isinstance(reg.sms(), AfricasTalkingSmsProvider)
    assert isinstance(reg.email(), SendGridEmailProvider)


def test_http_without_key_falls_back_to_console() -> None:
    # http requested but no API key set → console fallback (never silently broken).
    settings = make_settings(sms_provider="http", email_provider="http")
    with httpx.Client() as client:
        reg = build_channel_registry(settings, client)
    assert isinstance(reg.sms(), ConsoleSmsProvider)
    assert isinstance(reg.email(), ConsoleEmailProvider)
