"""SendGrid email provider — the production drop-in (config-gated ``NOTIFY_EMAIL_PROVIDER=http``).

Synchronous (runs in the worker). The API key comes from ``SecretStr`` env and is read only at the
call site — never logged. A network error or non-2xx raises :class:`SendError` (→ retry/backoff).
"""

from __future__ import annotations

import httpx

from app.channels.base import SendError, SendResult


class SendGridEmailProvider:
    name = "http"

    def __init__(
        self, client: httpx.Client, *, base_url: str, api_key: str, email_from: str
    ) -> None:
        self._client = client
        self._base = base_url.rstrip("/")
        self._api_key = api_key
        self._from = email_from

    def send(self, *, to: str, body: str, subject: str | None = None) -> SendResult:
        payload = {
            "personalizations": [{"to": [{"email": to}]}],
            "from": {"email": self._from},
            "subject": subject or "",
            "content": [{"type": "text/plain", "value": body}],
        }
        try:
            resp = self._client.post(
                f"{self._base}/v3/mail/send",
                json=payload,
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
        except httpx.HTTPError as exc:
            raise SendError(self.name, detail=str(exc)) from exc
        if resp.status_code >= 400:
            raise SendError(self.name, status=resp.status_code)
        return SendResult(provider=self.name, provider_ref=resp.headers.get("X-Message-Id", ""))
