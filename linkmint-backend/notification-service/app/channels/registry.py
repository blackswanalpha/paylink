"""Channel registry — the single swap point (console sandbox in dev/tests, HTTP vendor in prod).

The console providers are always the fallback; ``NOTIFY_SMS_PROVIDER=http`` /
``NOTIFY_EMAIL_PROVIDER=http`` (with the matching API key set) swap in Africa's Talking / SendGrid.
Built once in the Celery worker bootstrap with a sync ``httpx.Client``.
"""

from __future__ import annotations

import httpx

from app.channels.base import Provider
from app.channels.console import ConsoleEmailProvider, ConsoleSmsProvider
from app.channels.http_email import SendGridEmailProvider
from app.channels.http_sms import AfricasTalkingSmsProvider
from app.config import Settings
from app.domain.models import Channel


class ChannelRegistry:
    def __init__(self, sms: Provider, email: Provider) -> None:
        self._sms = sms
        self._email = email

    def sms(self) -> Provider:
        return self._sms

    def email(self) -> Provider:
        return self._email

    def for_channel(self, channel: str) -> Provider | None:
        if channel == Channel.SMS.value:
            return self._sms
        if channel == Channel.EMAIL.value:
            return self._email
        return None


def build_channel_registry(settings: Settings, client: httpx.Client) -> ChannelRegistry:
    fail = settings.console_fail_set()

    sms: Provider
    if settings.sms_provider == "http" and settings.africastalking_api_key is not None:
        sms = AfricasTalkingSmsProvider(
            client,
            base_url=settings.africastalking_base_url,
            username=settings.africastalking_username,
            api_key=settings.africastalking_api_key.get_secret_value(),
        )
    else:
        sms = ConsoleSmsProvider(fail)

    email: Provider
    if settings.email_provider == "http" and settings.sendgrid_api_key is not None:
        email = SendGridEmailProvider(
            client,
            base_url=settings.sendgrid_base_url,
            api_key=settings.sendgrid_api_key.get_secret_value(),
            email_from=settings.email_from,
        )
    else:
        email = ConsoleEmailProvider(fail)

    return ChannelRegistry(sms, email)
