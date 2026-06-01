"""Console (sandbox) channel providers — the Phase-1 default, the acceptance/verification path, and
the test double (no network, no real creds).

A send logs ONE redacted line and returns a deterministic ``provider_ref``. Recipients in
``fail_recipients`` (from ``NOTIFY_CONSOLE_FAIL_RECIPIENTS``) raise :class:`SendError` — the
deterministic hook that exercises retry/backoff/exhaustion end-to-end with no network.
"""

from __future__ import annotations

import hashlib

from app.channels.base import SendError, SendResult
from app.logging import get_logger
from app.redaction import mask_recipient

log = get_logger("notify.channel")


def _provider_ref(channel: str, to: str, body: str) -> str:
    digest = hashlib.sha1(
        f"{channel}|{to}|{body}".encode()
    ).hexdigest()  # noqa: S324 - non-crypto id
    return f"console-{digest[:12]}"


class _ConsoleProvider:
    name = "console"
    _channel = "console"

    def __init__(self, fail_recipients: frozenset[str] | None = None) -> None:
        self._fail = fail_recipients or frozenset()

    def send(self, *, to: str, body: str, subject: str | None = None) -> SendResult:
        if to in self._fail:
            masked = mask_recipient(self._channel, to)
            raise SendError(self.name, detail=f"forced failure for {masked}")
        log.info(
            "console_send",
            channel=self._channel,
            to=mask_recipient(self._channel, to),
            subject=subject,
            chars=len(body),
        )
        return SendResult(provider=self.name, provider_ref=_provider_ref(self._channel, to, body))


class ConsoleSmsProvider(_ConsoleProvider):
    _channel = "sms"


class ConsoleEmailProvider(_ConsoleProvider):
    _channel = "email"
