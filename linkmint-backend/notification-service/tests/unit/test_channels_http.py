"""HTTP channel drop-ins (Africa's Talking SMS, SendGrid email) via respx-mocked sync httpx."""

from __future__ import annotations

import httpx
import pytest
import respx

from app.channels.base import SendError
from app.channels.http_email import SendGridEmailProvider
from app.channels.http_sms import AfricasTalkingSmsProvider

AT_BASE = "https://api.africastalking.test/version1"
SG_BASE = "https://sendgrid.test"


@respx.mock
def test_africastalking_success() -> None:
    route = respx.post(f"{AT_BASE}/messaging").mock(
        return_value=httpx.Response(
            201,
            json={
                "SMSMessageData": {"Recipients": [{"messageId": "ATXid_9", "status": "Success"}]}
            },
        )
    )
    with httpx.Client() as client:
        p = AfricasTalkingSmsProvider(
            client, base_url=AT_BASE, username="sandbox", api_key="secret-key"
        )
        result = p.send(to="+254712345678", body="hi")
    assert result.provider_ref == "ATXid_9"
    # API key travels in the apiKey header, never the body.
    assert route.calls.last.request.headers["apiKey"] == "secret-key"


@respx.mock
def test_africastalking_non_2xx_raises() -> None:
    respx.post(f"{AT_BASE}/messaging").mock(return_value=httpx.Response(500))
    with httpx.Client() as client:
        p = AfricasTalkingSmsProvider(client, base_url=AT_BASE, username="sandbox", api_key="k")
        with pytest.raises(SendError) as exc:
            p.send(to="+254712345678", body="hi")
    assert exc.value.status == 500


@respx.mock
def test_sendgrid_success_returns_message_id() -> None:
    respx.post(f"{SG_BASE}/v3/mail/send").mock(
        return_value=httpx.Response(202, headers={"X-Message-Id": "msg-123"})
    )
    with httpx.Client() as client:
        p = SendGridEmailProvider(
            client, base_url=SG_BASE, api_key="sg-key", email_from="no-reply@x.io"
        )
        result = p.send(to="jane@example.com", body="hi", subject="Hello")
    assert result.provider == "http"
    assert result.provider_ref == "msg-123"


@respx.mock
def test_sendgrid_non_2xx_raises() -> None:
    respx.post(f"{SG_BASE}/v3/mail/send").mock(return_value=httpx.Response(401))
    with httpx.Client() as client:
        p = SendGridEmailProvider(client, base_url=SG_BASE, api_key="bad", email_from="x@x.io")
        with pytest.raises(SendError):
            p.send(to="jane@example.com", body="hi", subject="Hello")


@respx.mock
def test_network_error_raises_send_error() -> None:
    respx.post(f"{SG_BASE}/v3/mail/send").mock(side_effect=httpx.ConnectError("boom"))
    with httpx.Client() as client:
        p = SendGridEmailProvider(client, base_url=SG_BASE, api_key="k", email_from="x@x.io")
        with pytest.raises(SendError):
            p.send(to="jane@example.com", body="hi", subject="s")
