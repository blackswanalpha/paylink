from __future__ import annotations

import httpx
import pytest

from app.providers.http import HttpKycProvider
from app.providers.registry import build_registry
from tests._support import CALLBACK_SECRET, make_settings


@pytest.fixture
def http_client() -> httpx.AsyncClient:
    return httpx.AsyncClient()


def test_stub_always_present_and_default(http_client: httpx.AsyncClient) -> None:
    reg = build_registry(make_settings(), http_client)
    assert reg.get("stub") is not None
    assert reg.default().name == "stub"


def test_secret_lookup(http_client: httpx.AsyncClient) -> None:
    reg = build_registry(make_settings(), http_client)
    assert reg.secret_for("stub") == CALLBACK_SECRET
    assert reg.secret_for("unknown") == ""  # missing → empty → callbacks 401


def test_unknown_provider_returns_none(http_client: httpx.AsyncClient) -> None:
    reg = build_registry(make_settings(), http_client)
    assert reg.get("does-not-exist") is None


def test_http_provider_added_and_default_when_selected(
    http_client: httpx.AsyncClient,
) -> None:
    reg = build_registry(
        make_settings(kyc_provider="http", kyc_provider_url="https://vendor.example"),
        http_client,
    )
    http_provider = reg.get("http")
    assert isinstance(http_provider, HttpKycProvider)
    assert reg.default().name == "http"  # http is inserted first
    assert reg.get("stub") is not None  # stub still present


def test_http_selected_without_url_falls_back_to_stub(
    http_client: httpx.AsyncClient,
) -> None:
    reg = build_registry(make_settings(kyc_provider="http", kyc_provider_url=""), http_client)
    assert reg.get("http") is None
    assert reg.default().name == "stub"
