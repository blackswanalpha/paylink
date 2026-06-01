"""Africa's Talking SMS provider — production drop-in (``NOTIFY_SMS_PROVIDER=http``).

Synchronous (runs in the worker). The API key comes from ``SecretStr`` env and is read only at the
call site — never logged. A network error or non-2xx raises :class:`SendError` (→ retry/backoff).
"""

from __future__ import annotations

import httpx

from app.channels.base import SendError, SendResult


class AfricasTalkingSmsProvider:
    name = "http"

    def __init__(self, client: httpx.Client, *, base_url: str, username: str, api_key: str) -> None:
        self._client = client
        self._base = base_url.rstrip("/")
        self._username = username
        self._api_key = api_key

    def send(self, *, to: str, body: str, subject: str | None = None) -> SendResult:
        try:
            resp = self._client.post(
                f"{self._base}/messaging",
                data={"username": self._username, "to": to, "message": body},
                headers={
                    "apiKey": self._api_key,
                    "Accept": "application/json",
                    "Content-Type": "application/x-www-form-urlencoded",
                },
            )
        except httpx.HTTPError as exc:
            raise SendError(self.name, detail=str(exc)) from exc
        if resp.status_code >= 400:
            raise SendError(self.name, status=resp.status_code)
        ref = ""
        try:
            recipients = resp.json().get("SMSMessageData", {}).get("Recipients", [])
            if recipients:
                ref = str(recipients[0].get("messageId", ""))
        except (ValueError, AttributeError, IndexError):
            ref = ""
        return SendResult(provider=self.name, provider_ref=ref)
