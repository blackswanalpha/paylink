"""Best-effort client for notification-service's in-app inbox (FE work07).

On a PayLink lifecycle transition (created / verified / cancelled) the service posts an
address-scoped notification to ``POST /v1/notifications`` (the trusted-network intake; the gateway/
mesh terminates mTLS, an optional ``X-Internal-Token`` adds defense-in-depth, ADR-009). This mirrors
:class:`~app.compliance.client.ComplianceClient` but is **fire-and-forget**: any transport / non-2xx
failure is logged and swallowed so a notification hiccup never fails (or slows past its short
timeout) the PayLink operation. Non-custodial — moves no funds (A.1).
"""

from __future__ import annotations

from typing import Any

import httpx

from app.logging import get_logger

log = get_logger("paylink.notify")


class NotificationClient:
    def __init__(
        self,
        base_url: str,
        http: httpx.AsyncClient,
        *,
        internal_token: str | None = None,
        timeout: float = 3.0,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._http = http
        self._token = internal_token
        self._timeout = timeout

    async def notify(
        self,
        *,
        event_kind: str,
        recipient_addr: str,
        data: dict[str, Any],
        dedupe_id: str,
        title: str | None = None,
        body: str | None = None,
        href: str | None = None,
    ) -> None:
        """Post one in-app notification. Best-effort: never raises (logs + returns on failure).

        ``dedupe_id`` makes the notification idempotent at the inbox (the UNIQUE dedupe index), so a
        retried PayLink op or an at-least-once redelivery can't double-post.
        """
        payload: dict[str, Any] = {
            "event_kind": event_kind,
            "recipient_addr": recipient_addr,
            "data": {**data, "dedupe_id": dedupe_id},
        }
        if title is not None:
            payload["title"] = title
        if body is not None:
            payload["body"] = body
        if href is not None:
            payload["href"] = href

        headers = {"Content-Type": "application/json"}
        if self._token:
            headers["X-Internal-Token"] = self._token

        try:
            resp = await self._http.post(
                f"{self._base}/v1/notifications",
                json=payload,
                headers=headers,
                timeout=self._timeout,
            )
        except httpx.HTTPError as exc:
            log.warning("notify_unreachable", event_kind=event_kind, error=str(exc))
            return
        if resp.status_code >= 300:
            log.warning("notify_non_2xx", event_kind=event_kind, status=resp.status_code)
