"""Channel provider protocol + result/error types.

A ``Provider`` sends one rendered message and returns a :class:`SendResult` (a provider message
reference) or raises :class:`SendError` — the retryable failure the :class:`~app.delivery.runner.
DeliveryRunner` catches to schedule a backoff retry. The console providers are the Phase-1
default + the test double; the httpx drop-ins (Africa's Talking / SendGrid) are the production swap.

Sends are **synchronous** — they run in the Celery worker, not the async web app.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


class SendError(Exception):
    """A channel provider send failed (network error or non-2xx response) — retryable."""

    def __init__(self, provider: str, *, status: int | None = None, detail: str = "") -> None:
        self.provider = provider
        self.status = status
        self.detail = detail
        super().__init__(f"{provider} send failed (status={status}) {detail}".strip())


@dataclass(frozen=True)
class SendResult:
    """A provider's accept receipt for one message."""

    provider: str
    provider_ref: str


class Provider(Protocol):
    name: str

    def send(self, *, to: str, body: str, subject: str | None = None) -> SendResult: ...
