"""The provider registry — the single swap point (HTTP in prod, fakes in tests)."""

from __future__ import annotations

from app.providers.base import Provider


class ProviderRegistry:
    def __init__(self, providers: list[Provider]) -> None:
        self._providers: dict[str, Provider] = {p.entity_type: p for p in providers}

    def all(self) -> list[Provider]:
        """Every provider, in registration order (defines the search fan-out order)."""
        return list(self._providers.values())

    def provider_for(self, entity_type: str) -> Provider:
        return self._providers[entity_type]
