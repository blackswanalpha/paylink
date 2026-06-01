"""httpx-async providers — read each entity from its owning service's admin/internal endpoint.

These run on the trusted internal network with no service-to-service auth (the precedent of
payment-orchestrator → paylink-service). A 404 maps to ``None`` (absent); any other failure raises
:class:`UpstreamError`, which the search fan-out turns into a degraded group and the entity view
turns into a 502. Payloads are normalized into :class:`SearchHit` / :class:`EntityView`.
"""

from __future__ import annotations

from typing import Any

import httpx

from app.config import Settings
from app.domain.models import EntityView, SearchHit
from app.providers.base import UpstreamError
from app.providers.registry import ProviderRegistry

# A paylink id is a 0x-prefixed 32-byte hex hash (66 chars) — used to classify a search term.
_PAYLINK_ID_LEN = 66


class UpstreamClient:
    """Shared httpx-client wrapper: GET-JSON with the 404→None / error→raise contract."""

    def __init__(self, client: httpx.AsyncClient, base_url: str, service: str) -> None:
        self._client = client
        self._base = base_url.rstrip("/")
        self._service = service

    async def get_json(
        self, path: str, params: dict[str, Any] | None = None
    ) -> dict[str, Any] | None:
        try:
            resp = await self._client.get(self._base + path, params=params)
        except httpx.HTTPError as exc:
            raise UpstreamError(self._service) from exc
        if resp.status_code == 404:
            return None
        if resp.status_code >= 400:
            raise UpstreamError(self._service, status=resp.status_code)
        body = resp.json()
        return body if isinstance(body, dict) else None


def _user_hit(d: dict[str, Any]) -> SearchHit:
    return SearchHit(
        type="user",
        id=str(d.get("user_id", "")),
        label=str(d.get("email") or d.get("phone") or d.get("user_id", "")),
        status=d.get("status"),
        secondary={"kyc_tier": str(d.get("kyc_tier", ""))},
    )


def _merchant_hit(d: dict[str, Any]) -> SearchHit:
    return SearchHit(
        type="merchant",
        id=str(d.get("merchant_id", "")),
        label=str(d.get("business_name", "")),
        status=d.get("status"),
        secondary={
            "org_id": str(d.get("org_id", "")),
            "country": str(d.get("country", "")),
            "fee_tier": str(d.get("fee_tier", "")),
        },
    )


def _paylink_hit(d: dict[str, Any]) -> SearchHit:
    return SearchHit(
        type="paylink",
        id=str(d.get("pl_id", "")),
        label=str(d.get("pl_id", "")),
        status=d.get("status"),
        secondary={"amount": str(d.get("amount", "")), "currency": str(d.get("currency", ""))},
    )


def _payment_hit(d: dict[str, Any]) -> SearchHit:
    return SearchHit(
        type="payment",
        id=str(d.get("id", "")),
        label=str(d.get("id", "")),
        status=d.get("status"),
        secondary={"paylink_id": str(d.get("paylink_id", "")), "rail": str(d.get("rail", ""))},
    )


class HttpUserProvider:
    entity_type = "user"

    def __init__(self, client: UpstreamClient) -> None:
        self._c = client

    async def get(self, entity_id: str) -> EntityView | None:
        data = await self._c.get_json(f"/internal/admin/users/{entity_id}")
        return None if data is None else EntityView("user", entity_id, data)

    async def search(self, q: str, limit: int) -> list[SearchHit]:
        data = await self._c.get_json("/internal/admin/users", params={"q": q, "limit": limit})
        return [_user_hit(i) for i in (data or {}).get("items", [])]


class HttpMerchantProvider:
    entity_type = "merchant"

    def __init__(self, client: UpstreamClient) -> None:
        self._c = client

    async def get(self, entity_id: str) -> EntityView | None:
        data = await self._c.get_json(f"/internal/admin/merchants/{entity_id}")
        return None if data is None else EntityView("merchant", entity_id, data)

    async def search(self, q: str, limit: int) -> list[SearchHit]:
        data = await self._c.get_json("/internal/admin/merchants", params={"q": q, "limit": limit})
        return [_merchant_hit(i) for i in (data or {}).get("items", [])]


class HttpPaymentProvider:
    entity_type = "payment"

    def __init__(self, client: UpstreamClient) -> None:
        self._c = client

    async def get(self, entity_id: str) -> EntityView | None:
        data = await self._c.get_json(f"/internal/admin/payments/{entity_id}")
        return None if data is None else EntityView("payment", entity_id, data)

    async def search(self, q: str, limit: int) -> list[SearchHit]:
        data = await self._c.get_json("/internal/admin/payments", params={"q": q, "limit": limit})
        return [_payment_hit(i) for i in (data or {}).get("items", [])]


class HttpPaylinkProvider:
    """paylink-service has no free-text search — classify ``q`` onto its existing filters."""

    entity_type = "paylink"

    def __init__(self, client: UpstreamClient) -> None:
        self._c = client

    async def get(self, entity_id: str) -> EntityView | None:
        data = await self._c.get_json(f"/v1/paylinks/{entity_id}")
        return None if data is None else EntityView("paylink", entity_id, data)

    async def search(self, q: str, limit: int) -> list[SearchHit]:
        if q.startswith("0x") and len(q) == _PAYLINK_ID_LEN:
            view = await self.get(q)
            return [_paylink_hit(view.data)] if view is not None else []
        # Otherwise try the creator/receiver/status filters in turn; first non-empty wins.
        for param in ("creator", "receiver", "status"):
            data = await self._c.get_json("/v1/paylinks", params={param: q, "limit": limit})
            items = (data or {}).get("items", [])
            if items:
                return [_paylink_hit(i) for i in items]
        return []


def build_registry(settings: Settings, client: httpx.AsyncClient) -> ProviderRegistry:
    """Wire the four HTTP providers from the configured upstream URLs."""
    return ProviderRegistry(
        [
            HttpUserProvider(
                UpstreamClient(client, settings.identity_service_url, "identity-service")
            ),
            HttpMerchantProvider(
                UpstreamClient(client, settings.merchant_service_url, "merchant-onboarding")
            ),
            HttpPaylinkProvider(
                UpstreamClient(client, settings.paylink_service_url, "paylink-service")
            ),
            HttpPaymentProvider(
                UpstreamClient(client, settings.payment_orchestrator_url, "payment-orchestrator")
            ),
        ]
    )
